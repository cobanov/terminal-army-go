package svc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthService owns registration, login, JWT issuance, and session lookup.
// The signer itself lives in internal/auth; we use the TokenIssuer interface
// to avoid an import cycle.
type AuthService struct {
	app *App
}

// Sentinel errors so handlers can map to specific HTTP status codes.
var (
	ErrUsernameTaken    = errors.New("username is already taken")
	ErrEmailTaken       = errors.New("email is already registered")
	ErrInvalidUsername  = errors.New("username must be 3-32 chars: letters, digits, _ or -")
	ErrInvalidEmail     = errors.New("invalid email address")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrInvalidLogin     = errors.New("invalid username or password")
	ErrSessionExpired   = errors.New("session expired")
	ErrDevicePending    = errors.New("pending")
)

const (
	deviceAuthTTLSeconds      = 600
	deviceAuthPollingInterval = 2
)

// Register creates a new user, signs an initial JWT, and stores the device
// session row that owns it. Returns the auth result the TUI/web client uses.
func (s *AuthService) Register(ctx context.Context, username, email, password string) (*AuthResult, error) {
	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))

	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	if _, err := s.app.Queries.GetUserByUsername(ctx, username); err == nil {
		return nil, ErrUsernameTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if _, err := s.app.Queries.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), s.app.Cfg.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	userID, err := s.app.Queries.CreateUser(ctx, username, email, string(hashBytes))
	if err != nil {
		// Race: another concurrent registration grabbed the name. Surface a
		// friendly error instead of leaking the unique-violation text.
		if isUniqueViolation(err) {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}

	return s.issueSession(ctx, userID)
}

// Login verifies credentials with bcrypt, mints a JWT, and stores a device
// session. Username comparison is case-sensitive to match the unique index.
func (s *AuthService) Login(ctx context.Context, username, password string) (*AuthResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidLogin
	}
	u, err := s.app.Queries.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Spend bcrypt time anyway to avoid leaking which usernames exist.
			// Compare against a hash generated at the SAME cost as real hashes
			// so unknown-username timing matches known-username timing.
			_ = bcrypt.CompareHashAndPassword(s.timingDummyHash(), []byte(password))
			return nil, ErrInvalidLogin
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidLogin
	}
	return s.issueSession(ctx, u.ID)
}

// StartDeviceAuth creates a short-lived auth code that the CLI can poll while
// the user signs in through the browser.
func (s *AuthService) StartDeviceAuth(ctx context.Context) (*DeviceAuthStart, error) {
	code, err := newBrowserAuthCode()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(deviceAuthTTLSeconds * time.Second)
	if _, err := s.app.Queries.CreatePendingDeviceSession(ctx, code, expiresAt); err != nil {
		return nil, err
	}
	return &DeviceAuthStart{
		AuthCode:        code,
		ExpiresIn:       deviceAuthTTLSeconds,
		PollingInterval: deviceAuthPollingInterval,
	}, nil
}

// PollDeviceAuth returns the bound token once browser auth completes. A
// pending code returns ErrDevicePending; an expired code is deleted.
func (s *AuthService) PollDeviceAuth(ctx context.Context, code string) (*DeviceAuthPoll, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, store.ErrNotFound
	}
	row, err := s.app.Queries.GetDeviceSessionByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if row.ExpiresAt.Before(now) {
		_ = s.app.Queries.DeleteDeviceSessionByCode(ctx, code)
		return nil, ErrSessionExpired
	}
	if row.Token == nil || *row.Token == "" {
		return nil, ErrDevicePending
	}
	token := *row.Token
	_ = s.app.Queries.DeleteDeviceSessionByCode(ctx, code)
	return &DeviceAuthPoll{Token: token}, nil
}

// BindDeviceAuth attaches a newly issued JWT to a pending browser auth code.
// It returns false when the code is absent, expired, or already consumed.
func (s *AuthService) BindDeviceAuth(ctx context.Context, code, token string, userID int64) bool {
	code = strings.TrimSpace(code)
	if code == "" || token == "" || userID == 0 {
		return false
	}
	session, err := s.app.Queries.GetDeviceSessionByToken(ctx, token)
	if err != nil {
		return false
	}
	return s.app.Queries.BindDeviceSessionToken(ctx, code, token, userID, session.ExpiresAt) == nil
}

// Logout deletes the device session row keyed by the bearer token. We do not
// invalidate the JWT signature (HS256 tokens have no revocation list); the
// session-row lookup in ResolveSession is what makes logout meaningful.
func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.app.Queries.DeleteDeviceSessionByToken(ctx, token)
}

// ResolveSession is the SessionLookup implementation used by the auth
// middleware. It verifies the JWT, looks up the device session row, fetches
// the user record, and bumps last_seen_at.
func (s *AuthService) ResolveSession(ctx context.Context, token string) (*Session, error) {
	if s.app.Tokens == nil {
		return nil, errors.New("auth: token issuer not configured")
	}
	uid, _, err := s.app.Tokens.Verify(token)
	if err != nil {
		return nil, err
	}

	row, err := s.app.Queries.GetDeviceSessionByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if row.UserID == nil || *row.UserID != uid {
		return nil, ErrInvalidLogin
	}
	now := time.Now().UTC()
	if !row.ExpiresAt.IsZero() && row.ExpiresAt.Before(now) {
		// Best-effort cleanup; ignore deletion error.
		_ = s.app.Queries.DeleteDeviceSessionByToken(ctx, token)
		return nil, ErrSessionExpired
	}

	user, err := s.app.Queries.GetUserByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	// Bump last_seen_at asynchronously so the auth path stays fast even when
	// the db ping is slow. We deliberately swallow the error here.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.app.Queries.UpdateUserLastSeen(bgCtx, uid, time.Now().UTC())
	}()

	return &Session{
		ID:        row.ID,
		UserID:    uid,
		User:      mapUser(user),
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

// issueSession mints a JWT, stores its device_sessions row, and returns the
// AuthResult. Runs in a transaction so a partial insert never leaves an
// orphaned token.
func (s *AuthService) issueSession(ctx context.Context, userID int64) (*AuthResult, error) {
	if s.app.Tokens == nil {
		return nil, errors.New("auth: token issuer not configured")
	}
	code, err := newSessionCode()
	if err != nil {
		return nil, err
	}
	token, expiresAt, err := s.app.Tokens.Issue(userID, code)
	if err != nil {
		return nil, err
	}

	var user *store.User
	err = store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		if _, err := qtx.CreateDeviceSession(ctx, code, token, userID, expiresAt); err != nil {
			return err
		}
		u, err := qtx.GetUserByID(ctx, userID)
		if err != nil {
			return err
		}
		user = u
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &AuthResult{Token: token, User: mapUser(user)}, nil
}

// mapUser converts the store row to the public svc.User.
func mapUser(u *store.User) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:                u.ID,
		Username:          u.Username,
		Email:             u.Email,
		Role:              u.Role,
		CurrentUniverseID: u.CurrentUniverseID,
		CreatedAt:         u.CreatedAt,
		LastSeenAt:        u.LastSeenAt,
	}
}

// dummyHash is a precomputed bcrypt hash (cost 10) used as a fallback for the
// login timing-equalizer. It is never matched against a real password.
var dummyHash = []byte("$2a$10$7EqJtq98hPqEX7fNZaFWoO.7AYqgRZpC8rGjJUMSh2RXc3XSXfh.W")

// dummyHashOnce lazily builds a timing-equalizer hash at the configured bcrypt
// cost so an unknown-username login spends the same time as a known one.
var (
	dummyHashOnce   sync.Once
	dummyHashAtCost []byte
)

// timingDummyHash returns a bcrypt hash generated at the configured cost,
// computed once per process. Falls back to the cost-10 constant on error.
func (s *AuthService) timingDummyHash() []byte {
	dummyHashOnce.Do(func() {
		h, err := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), s.app.Cfg.BcryptCost)
		if err != nil {
			dummyHashAtCost = dummyHash
			return
		}
		dummyHashAtCost = h
	})
	return dummyHashAtCost
}

func newSessionCode() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newBrowserAuthCode() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validateUsername(name string) error {
	if n := len(name); n < 3 || n > 32 {
		return ErrInvalidUsername
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return ErrInvalidUsername
	}
	return nil
}

func validateEmail(email string) error {
	if len(email) < 5 || len(email) > 255 {
		return ErrInvalidEmail
	}
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return ErrInvalidEmail
	}
	if strings.IndexByte(email[at+1:], '.') == -1 {
		return ErrInvalidEmail
	}
	return nil
}

func validatePassword(pw string) error {
	if len(pw) < 8 {
		return ErrPasswordTooShort
	}
	return nil
}

// isUniqueViolation matches Postgres unique constraint errors without
// depending on the pgx package here directly. The error text is stable.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "SQLSTATE 23505") ||
		strings.Contains(err.Error(), "unique constraint")
}
