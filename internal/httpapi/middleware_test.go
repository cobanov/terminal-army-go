package httpapi

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealIPFromTrustedProxy(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		wantHost   string
	}{
		{"trusted loopback proxy honors XFF", "127.0.0.1:5000", "203.0.113.9, 10.0.0.1", "203.0.113.9"},
		{"trusted private proxy honors XFF", "10.0.0.5:5000", "198.51.100.7", "198.51.100.7"},
		{"public peer ignores spoofed XFF", "203.0.113.50:5000", "10.9.9.9", "203.0.113.50"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			var gotHost string
			realIPFromTrustedProxy(http.HandlerFunc(func(_ http.ResponseWriter, rr *http.Request) {
				gotHost, _, _ = net.SplitHostPort(rr.RemoteAddr)
			})).ServeHTTP(httptest.NewRecorder(), r)
			if gotHost != tc.wantHost {
				t.Fatalf("client host = %q, want %q", gotHost, tc.wantHost)
			}
		})
	}
}
