// Package token signs and verifies the short-lived access token, and makes the
// opaque refresh token. The access token carries only who (sub) and which
// sign-in (sid); the role and permissions are read from the database on every
// request, so a role change shows at once.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalid is any token that must not be trusted: bad signature, wrong
// algorithm, issuer or audience, expired, or missing a claim.
var ErrInvalid = errors.New("invalid token")

// leeway absorbs clock differences between instances.
const leeway = 5 * time.Second

type Config struct {
	Secret    []byte // HS256 key, at least 32 bytes (config checks)
	Issuer    string
	Audience  string
	AccessTTL time.Duration
}

// Identity is what a valid access token proves.
type Identity struct {
	UserID    int64
	SessionID string // the refresh-token family (sid)
	IssuedAt  time.Time
}

type Manager struct {
	cfg Config
	now func() time.Time
}

func New(cfg Config) *Manager { return &Manager{cfg: cfg, now: time.Now} }

// claims adds two private claims. iatm repeats iat in whole milliseconds as an
// integer: users.tokens_valid_after has sub-second precision, and iat, a float
// on the wire, loses a millisecond to rounding on the way back.
type claims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
	IssuedMs  int64  `json:"iatm"`
}

// Issue signs an access token for the user and sign-in, and returns when it expires.
func (m *Manager) Issue(userID int64, sessionID string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.cfg.AccessTTL)
	jti, err := Random(8)
	if err != nil {
		return "", time.Time{}, err
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    m.cfg.Issuer,
			Audience:  jwt.ClaimStrings{m.cfg.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        jti,
		},
		SessionID: sessionID,
		IssuedMs:  now.UnixMilli(),
	}).SignedString(m.cfg.Secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return raw, exp, nil
}

// Parse verifies raw. The algorithm is pinned to HS256, so a token that names
// another one ("none", RS256 with the secret as key) is refused.
func (m *Manager) Parse(raw string) (Identity, error) {
	var c claims
	_, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return m.cfg.Secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.cfg.Issuer),
		jwt.WithAudience(m.cfg.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(leeway),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil || id <= 0 || c.SessionID == "" || c.IssuedMs <= 0 {
		return Identity{}, fmt.Errorf("%w: missing claim", ErrInvalid)
	}
	return Identity{UserID: id, SessionID: c.SessionID, IssuedAt: time.UnixMilli(c.IssuedMs)}, nil
}

// Random returns n random bytes, URL-safe base64 encoded.
func Random(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Hash is what the database stores for a refresh token.
func Hash(refresh string) string {
	sum := sha256.Sum256([]byte(refresh))
	return hex.EncodeToString(sum[:])
}
