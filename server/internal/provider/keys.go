// Package provider owns the model providers whose credentials the server
// holds on behalf of the runtimes: storage of the secret, discovery of the
// models a key unlocks, and injection of the right environment for an agent.
//
// Keys never reach the browser. The API only ever returns a masked form.
package provider

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

// ErrNoKey is returned when a reference has no stored secret.
var ErrNoKey = errors.New("no key stored for that reference")

const keyringService = "ai-skope"

// Keystore stores provider secrets outside the database.
type Keystore interface {
	Set(ref, secret string) error
	Get(ref string) (string, error)
	Delete(ref string) error
	Backend() string
}

// NewKeystore returns the OS keychain when it is usable, and an encrypted
// file in the state directory otherwise (headless Linux, locked keychains).
// AISS_KEYSTORE=file forces the fallback.
func NewKeystore(stateDir string) Keystore {
	if os.Getenv("AISS_KEYSTORE") != "file" {
		k := &keyringStore{}
		if err := k.probe(); err == nil {
			return k
		} else {
			slog.Warn("OS keychain unavailable, using encrypted file store", "err", err)
		}
	}
	return newFileStore(stateDir)
}

type keyringStore struct{}

func (k *keyringStore) probe() error {
	const probeRef = "__probe__"
	if err := keyring.Set(keyringService, probeRef, "ok"); err != nil {
		return err
	}
	_, err := keyring.Get(keyringService, probeRef)
	_ = keyring.Delete(keyringService, probeRef)
	return err
}

func (k *keyringStore) Set(ref, secret string) error { return keyring.Set(keyringService, ref, secret) }

func (k *keyringStore) Get(ref string) (string, error) {
	v, err := keyring.Get(keyringService, ref)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNoKey
	}
	return v, err
}

func (k *keyringStore) Delete(ref string) error {
	err := keyring.Delete(keyringService, ref)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func (k *keyringStore) Backend() string { return "keychain" }

// fileStore keeps secrets in an AES-GCM encrypted file. The master key lives
// beside it with 0600 permissions: weaker than the OS keychain, and only used
// when no keychain is available.
type fileStore struct {
	mu       sync.Mutex
	dir      string
	dataPath string
	keyPath  string
}

func newFileStore(dir string) *fileStore {
	return &fileStore{
		dir:      dir,
		dataPath: filepath.Join(dir, "keys.enc"),
		keyPath:  filepath.Join(dir, "keys.master"),
	}
}

func (f *fileStore) Backend() string { return "file" }

func (f *fileStore) master() ([]byte, error) {
	if b, err := os.ReadFile(f.keyPath); err == nil {
		key, err := base64.StdEncoding.DecodeString(string(b))
		if err == nil && len(key) == 32 {
			return key, nil
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(f.keyPath, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (f *fileStore) load() (map[string]string, error) {
	out := map[string]string{}
	raw, err := os.ReadFile(f.dataPath)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	key, err := f.master()
	if err != nil {
		return nil, err
	}
	blob, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, errors.New("key store is corrupt")
	}
	plain, err := gcm.Open(nil, blob[:gcm.NonceSize()], blob[gcm.NonceSize():], nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (f *fileStore) save(m map[string]string) error {
	key, err := f.master()
	if err != nil {
		return err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return err
	}
	plain, err := json.Marshal(m)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	blob := gcm.Seal(nonce, nonce, plain, nil)
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(f.dataPath, []byte(base64.StdEncoding.EncodeToString(blob)), 0o600)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (f *fileStore) Set(ref, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.load()
	if err != nil {
		return err
	}
	m[ref] = secret
	return f.save(m)
}

func (f *fileStore) Get(ref string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.load()
	if err != nil {
		return "", err
	}
	v, ok := m[ref]
	if !ok {
		return "", ErrNoKey
	}
	return v, nil
}

func (f *fileStore) Delete(ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, err := f.load()
	if err != nil {
		return err
	}
	delete(m, ref)
	return f.save(m)
}

// Mask renders a secret the way the UI shows it: enough to recognise, never
// enough to use.
func Mask(secret string) string {
	if secret == "" {
		return ""
	}
	r := []rune(secret)
	if len(r) <= 10 {
		return "…" + string(r[len(r)-2:])
	}
	return string(r[:6]) + "…" + string(r[len(r)-4:])
}
