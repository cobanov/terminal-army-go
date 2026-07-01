package config

import "testing"

func TestLoadServerDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("TARMY_HTTP_ADDR", "")
	t.Setenv("TARMY_PUBLIC_URL", "")
	t.Setenv("TARMY_SERVER_NAME", "")
	t.Setenv("TARMY_SERVER_DESCRIPTION", "")
	t.Setenv("TARMY_SERVER_MAX_USERS", "")
	t.Setenv("TARMY_JWT_SECRET", "this-secret-is-long-enough")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ServerName != "s1" {
		t.Fatalf("ServerName = %q, want s1", cfg.ServerName)
	}
	if cfg.ServerDesc != "the first universe" {
		t.Fatalf("ServerDesc = %q", cfg.ServerDesc)
	}
	if cfg.ServerMaxUsers != 5000 {
		t.Fatalf("ServerMaxUsers = %d, want 5000", cfg.ServerMaxUsers)
	}
}

func TestLoadRejectsInsecureDefaultSecret(t *testing.T) {
	// Empty env falls back to the built-in default, which must be rejected.
	t.Setenv("TARMY_JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load should reject the built-in default JWT secret")
	}
}
