package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload we issue at login.
type Claims struct {
	UserID      int64  `json:"uid"`
	SessionCode string `json:"sid"`
	jwt.RegisteredClaims
}

// Signer signs and verifies HS256 JWTs with a static secret.
type Signer struct {
	secret []byte
	ttl    time.Duration
}

// NewSigner constructs a Signer. Panics if secret is empty (config layer
// already enforces a minimum length so this is defence in depth).
func NewSigner(secret string, ttl time.Duration) *Signer {
	if secret == "" {
		panic("auth: empty JWT secret")
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Signer{secret: []byte(secret), ttl: ttl}
}

// TTL exposes the configured token lifetime so callers can populate the
// matching device_sessions.expires_at column.
func (s *Signer) TTL() time.Duration { return s.ttl }

// Issue mints a new signed token for the given user and session code.
// Returns the token string and its expiry instant.
func (s *Signer) Issue(userID int64, sessionCode string) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(s.ttl)
	claims := Claims{
		UserID:      userID,
		SessionCode: sessionCode,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "tarmy",
			Subject:   fmt.Sprintf("user:%d", userID),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// ErrInvalidToken is returned by Verify when parsing or signature checks fail.
var ErrInvalidToken = errors.New("invalid token")

// Verify parses and validates a token. Implements svc.TokenIssuer.Verify by
// returning the embedded user id and session code rather than the raw claims.
func (s *Signer) Verify(raw string) (int64, string, error) {
	if raw == "" {
		return 0, "", ErrInvalidToken
	}
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected alg %q", t.Method.Alg())
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return 0, "", ErrInvalidToken
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok {
		return 0, "", ErrInvalidToken
	}
	return c.UserID, c.SessionCode, nil
}

// NewSessionCode returns a random 32-character hex string used as the
// device_sessions.code primary lookup. We store this in the JWT and in the
// row so revocation works by deleting the row, not by rotating the secret.
func NewSessionCode() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
