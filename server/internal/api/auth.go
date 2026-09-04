package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ai-skope/aiss/internal/store"
)

type ctxKey int

const pairingKey ctxKey = iota

// auth requires a bearer token from a live pairing, and — when the request
// carries an Origin, as every browser request does — that the origin is one
// this server was paired with. A page on the open web therefore cannot use a
// token even if it somehow obtained one.
func (s *Server) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			writeErr(w, http.StatusUnauthorized, "unauthorized",
				"Pair this browser with the AI Skope Server first.")
			return
		}
		p, err := s.DB.PairingByToken(token)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "That pairing is no longer valid.")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !s.originAllowed(origin) {
			writeErr(w, http.StatusForbidden, "bad_origin", "That origin is not paired with this server.")
			return
		}
		ctx := context.WithValue(r.Context(), pairingKey, p)
		next(w, r.WithContext(ctx))
	})
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// pairingOf returns the pairing that authenticated the request.
func pairingOf(r *http.Request) *store.Pairing {
	p, _ := r.Context().Value(pairingKey).(*store.Pairing)
	return p
}

// originAllowed reports whether an Origin header belongs to a paired
// extension (or to localhost while running in dev mode).
func (s *Server) originAllowed(origin string) bool {
	if s.Cfg.DevMode && (strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:")) {
		return true
	}
	pairings, err := s.DB.Pairings()
	if err != nil {
		return false
	}
	for _, p := range pairings {
		if p.RevokedAt == 0 && strings.EqualFold(p.Origin, origin) {
			return true
		}
	}
	return false
}

// cors answers preflights and echoes the origin only for paired extensions.
// Pairing itself is allowed from any chrome-extension:// origin, since that is
// the request in which the extension first identifies itself.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			allowed := s.originAllowed(origin)
			if !allowed && r.URL.Path == "/v1/pair" && strings.HasPrefix(origin, "chrome-extension://") {
				allowed = true
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.Header().Set("Vary", "Origin")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limiter is a small fixed-window rate limiter, applied to the endpoints that
// spawn processes so a runaway client cannot fork-bomb the machine.
type limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	seen   map[string]*bucket
}

type bucket struct {
	count int
	reset time.Time
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{limit: limit, window: window, seen: map[string]*bucket{}}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.seen[key]
	if !ok || now.After(b.reset) {
		l.seen[key] = &bucket{count: 1, reset: now.Add(l.window)}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}
