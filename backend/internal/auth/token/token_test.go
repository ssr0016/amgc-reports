package token

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func newManager(now time.Time) *Manager {
	m := New(Config{Secret: secret, Issuer: "iss", Audience: "aud", AccessTTL: 10 * time.Minute})
	m.now = func() time.Time { return now }
	return m
}

func TestIssueParse_RoundTrip(t *testing.T) {
	now := time.Now()
	m := newManager(now)
	raw, exp, err := m.Issue(42, "fam")
	require.NoError(t, err)
	require.WithinDuration(t, now.Add(10*time.Minute), exp, time.Millisecond)

	id, err := m.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, int64(42), id.UserID)
	require.Equal(t, "fam", id.SessionID)
	require.WithinDuration(t, now, id.IssuedAt, 2*time.Millisecond, "iatm keeps millisecond precision")
}

func TestParse_Refuses(t *testing.T) {
	now := time.Now()
	good, _, err := newManager(now).Issue(1, "fam")
	require.NoError(t, err)

	sign := func(method jwt.SigningMethod, key any, c claims) string {
		raw, err := jwt.NewWithClaims(method, c).SignedString(key)
		require.NoError(t, err)
		return raw
	}
	base := func() claims {
		return claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "1", Issuer: "iss", Audience: jwt.ClaimStrings{"aud"},
				IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			},
			SessionID: "fam", IssuedMs: now.UnixMilli(),
		}
	}
	with := func(f func(*claims)) claims { c := base(); f(&c); return c }

	tests := map[string]string{
		"garbage":       "not-a-token",
		"empty":         "",
		"tampered":      good[:len(good)-2] + "xx",
		"wrong secret":  sign(jwt.SigningMethodHS256, []byte("another-secret-another-secret-123"), base()),
		"alg none":      sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, base()),
		"other hs alg":  sign(jwt.SigningMethodHS512, secret, base()),
		"wrong issuer":  sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.Issuer = "evil" })),
		"wrong aud":     sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.Audience = jwt.ClaimStrings{"other"} })),
		"expired":       sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute)) })),
		"no expiry":     sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.ExpiresAt = nil })),
		"no iatm":       sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.IssuedMs = 0 })),
		"no sid":        sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.SessionID = "" })),
		"bad subject":   sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.Subject = "abc" })),
		"zero subject":  sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.Subject = "0" })),
		"issued future": sign(jwt.SigningMethodHS256, secret, with(func(c *claims) { c.IssuedAt = jwt.NewNumericDate(now.Add(time.Hour)) })),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newManager(now).Parse(raw)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInvalid))
		})
	}
}

func TestParse_ExpiresAfterTTL(t *testing.T) {
	now := time.Now()
	raw, _, err := newManager(now).Issue(1, "fam")
	require.NoError(t, err)
	_, err = newManager(now.Add(10*time.Minute + 2*time.Second)).Parse(raw)
	require.NoError(t, err, "inside the leeway")
	_, err = newManager(now.Add(10*time.Minute + 30*time.Second)).Parse(raw)
	require.ErrorIs(t, err, ErrInvalid)
}

func TestHash_IsStableAndNotTheToken(t *testing.T) {
	r, err := Random(32)
	require.NoError(t, err)
	require.Equal(t, Hash(r), Hash(r))
	require.NotEqual(t, r, Hash(r))
	require.Len(t, Hash(r), 64)
}

// iat is a float on the wire; it must still come back exactly on the millisecond,
// or a token signed right after a revocation could look older than it.
func TestParse_IssuedAtIsOnTheMillisecond(t *testing.T) {
	for i := 0; i < 200; i++ {
		now := time.Now().Add(time.Duration(i) * 7919 * time.Microsecond)
		raw, _, err := newManager(now).Issue(1, "fam")
		require.NoError(t, err)
		id, err := newManager(now).Parse(raw)
		require.NoError(t, err)
		require.True(t, id.IssuedAt.Equal(now.Truncate(time.Millisecond)), "iat %v != %v ", id.IssuedAt.UnixNano(), now.Truncate(time.Millisecond).UnixNano())
	}
}
