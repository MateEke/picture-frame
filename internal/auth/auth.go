// Package auth provides password hashing and signed session cookies for the
// admin UI. The signing key is derived from the password hash, so changing or
// clearing the password invalidates every outstanding cookie.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// DefaultTTL is how long an issued session cookie stays valid.
const DefaultTTL = 30 * 24 * time.Hour

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckPassword reports whether plain matches the bcrypt hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Authenticator issues and verifies signed session cookies.
type Authenticator struct {
	TTL time.Duration
}

// New returns an Authenticator with the default session TTL.
func New() *Authenticator {
	return &Authenticator{TTL: DefaultTTL}
}

// Issue returns a cookie value valid for a.TTL, signed under passwordHash.
func (a *Authenticator) Issue(passwordHash string) string {
	exp := time.Now().Add(a.TTL).Unix()
	return a.sign(passwordHash, exp)
}

// Verify reports whether cookie is a valid, unexpired cookie for passwordHash.
func (a *Authenticator) Verify(passwordHash, cookie string) bool {
	expStr, _, ok := strings.Cut(cookie, ".")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	expected := a.sign(passwordHash, exp)
	if subtle.ConstantTimeCompare([]byte(cookie), []byte(expected)) != 1 {
		return false
	}
	return time.Now().Unix() < exp
}

// sign builds the "<expiry>.<base64url-hmac>" token, keyed by SHA-256 of the hash.
func (a *Authenticator) sign(passwordHash string, exp int64) string {
	key := sha256.Sum256([]byte(passwordHash))
	mac := hmac.New(sha256.New, key[:])
	expStr := strconv.FormatInt(exp, 10)
	mac.Write([]byte(expStr))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return expStr + "." + sig
}
