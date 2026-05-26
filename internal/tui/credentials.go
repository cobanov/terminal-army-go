// Credential cache for the TUI. We store the JWT in a 0600 file under the
// user's config dir so the player does not need to re-login every time the
// client launches. Tokens are not encrypted - the threat model is "stop the
// next process on this machine from grabbing my token by reading argv", not
// "defend against disk-level attackers". The file is best-effort: errors
// reading or writing it never block the player from logging in interactively.
package tui

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
)

// CachedCreds is what we persist to disk.
type CachedCreds struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	Username  string `json:"username"`
}

// credsDir resolves the per-user config directory and ensures it exists.
// Returns the directory path or an error if neither $XDG_CONFIG_HOME nor a
// platform default can be resolved.
func credsDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "tarmy")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// credsFile returns the on-disk path for the given serverURL. Each server URL
// gets its own file so a player can hop between dev and prod without
// stomping their other token.
func credsFile(serverURL string) (string, error) {
	dir, err := credsDir()
	if err != nil {
		return "", err
	}
	// url.QueryEscape gives us a filename that round-trips on every FS.
	safe := url.QueryEscape(serverURL)
	if safe == "" {
		safe = "default"
	}
	return filepath.Join(dir, safe+".json"), nil
}

// LoadCreds returns the cached credentials for serverURL or nil if none exist.
// Any read or unmarshal error is treated as "no credentials".
func LoadCreds(serverURL string) *CachedCreds {
	path, err := credsFile(serverURL)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	var c CachedCreds
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}
	if c.Token == "" {
		return nil
	}
	return &c
}

// SaveCreds persists c to disk under the server URL it carries. Returns an
// error if the file cannot be written; callers may log and continue.
func SaveCreds(c *CachedCreds) error {
	if c == nil || c.Token == "" || c.ServerURL == "" {
		return errors.New("incomplete credentials")
	}
	path, err := credsFile(c.ServerURL)
	if err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ClearCreds removes the cached credentials for serverURL. Returns nil if no
// file exists - the operation is idempotent.
func ClearCreds(serverURL string) error {
	path, err := credsFile(serverURL)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
