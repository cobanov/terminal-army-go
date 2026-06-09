package svc

import "testing"

func TestNewBrowserAuthCode(t *testing.T) {
	a, err := newBrowserAuthCode()
	if err != nil {
		t.Fatalf("newBrowserAuthCode returned error: %v", err)
	}
	b, err := newBrowserAuthCode()
	if err != nil {
		t.Fatalf("newBrowserAuthCode returned error: %v", err)
	}
	if a == "" || b == "" {
		t.Fatal("auth code should not be empty")
	}
	if a == b {
		t.Fatal("auth codes should be random")
	}
	for _, ch := range a + b {
		if ch == '/' || ch == '+' || ch == '=' {
			t.Fatalf("auth code contains non URL-safe character %q", ch)
		}
	}
}
