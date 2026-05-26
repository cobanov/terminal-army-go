// Package client is a thin HTTP wrapper around the tarmy REST API. The TUI
// uses it through tea.Cmd functions; all methods are blocking and return
// either the decoded payload or an error - the caller wraps them in a Cmd.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client holds the server base URL, an auth token, and an http.Client.
// Zero value is not usable; construct via New.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New constructs a Client pointed at baseURL.
func New(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// SetToken installs a bearer token for subsequent requests.
func (c *Client) SetToken(token string) { c.token = token }

// Token returns the currently installed bearer token.
func (c *Client) Token() string { return c.token }

// BaseURL returns the server URL the client targets.
func (c *Client) BaseURL() string { return c.baseURL }

// APIError carries the server's structured error body alongside the status code.
type APIError struct {
	Status  int    `json:"-"`
	Message string `json:"error"`
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("http %d", e.Status)
	}
	return fmt.Sprintf("http %d: %s", e.Status, e.Message)
}

// do builds and sends a request. When body is non-nil it is JSON-encoded.
// out is JSON-decoded from the response when non-nil and status is 2xx.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{Status: resp.StatusCode}
		_ = json.Unmarshal(data, apiErr)
		if apiErr.Message == "" {
			apiErr.Message = strings.TrimSpace(string(data))
		}
		return apiErr
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

// IsUnauthorized reports whether err is a 401 from the API.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusUnauthorized
	}
	return false
}
