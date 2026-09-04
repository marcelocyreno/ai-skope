package config

import (
	"os"
	"path/filepath"
)

const appDir = "ai-skope"

func xdg(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return filepath.Join(v, appDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, filepath.FromSlash(def), appDir)
}

// ConfigDir holds config.yaml.
func ConfigDir() string { return xdg("XDG_CONFIG_HOME", ".config") }

// DataDir holds the SQLite database.
func DataDir() string { return xdg("XDG_DATA_HOME", ".local/share") }

// StateDir holds logs and the pid file.
func StateDir() string { return xdg("XDG_STATE_HOME", ".local/state") }

// ConfigFile is the path of the YAML configuration file.
func ConfigFile() string { return filepath.Join(ConfigDir(), "config.yaml") }

// DBFile is the path of the SQLite database.
func DBFile() string { return filepath.Join(DataDir(), "aiss.db") }

// LogFile is the path of the rotating log file.
func LogFile() string { return filepath.Join(StateDir(), "aiss.log") }

// PIDFile is the path of the pid file written by `aiss start`.
func PIDFile() string { return filepath.Join(StateDir(), "aiss.pid") }

// EnsureDirs creates every directory the server writes to.
func EnsureDirs() error {
	for _, d := range []string{ConfigDir(), DataDir(), StateDir()} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	return nil
}
