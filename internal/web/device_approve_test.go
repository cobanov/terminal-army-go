package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

// The device-approval page is the security chokepoint that replaced silent
// GET binding, so make sure it renders with a CSRF field, the code, and a POST
// form to the approval endpoint.
func TestDeviceApproveTemplateRenders(t *testing.T) {
	var buf bytes.Buffer
	data := viewData{
		Title: "link device",
		CSRF:  "csrf-token-xyz",
		User:  &svc.User{Username: "alice"},
		Form:  map[string]string{"code": "ABC123"},
	}
	if err := render(&buf, "device_approve", data); err != nil {
		t.Fatalf("render device_approve: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ABC123", `action="/device/approve`, `name="csrf"`, "csrf-token-xyz", "alice", "method=\"post\""} {
		if !strings.Contains(out, want) {
			t.Errorf("device_approve output missing %q", want)
		}
	}
}
