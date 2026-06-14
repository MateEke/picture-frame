package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/auth"
	"github.com/MateEke/picture-frame/internal/config"
)

const sessionCookieName = "pf_session"

// failedPasswordDelay pads failed checks (under the mutex) to slow brute force.
const failedPasswordDelay = 500 * time.Millisecond

// checkPasswordGated serializes bcrypt: parallel guesses would peg the Pi's
// single core and multiply brute-force throughput.
func (s *server) checkPasswordGated(hash, plain string) bool {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if auth.CheckPassword(hash, plain) {
		return true
	}
	time.Sleep(failedPasswordDelay)
	return false
}

// hashPasswordGated runs the equally-expensive bcrypt hash under the same mutex.
func (s *server) hashPasswordGated(plain string) (string, error) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	return auth.HashPassword(plain)
}

// gatedPath: /api/* needs a session, except /api/auth/* (login and recovery).
func gatedPath(path string) bool {
	return strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/api/auth/")
}

// kioskExempt marks a route the on-device kiosk fetches over loopback without
// a cookie (remote LAN clients still need one). Called next to the route's
// registration so the exemption can't drift from the route. Startup-only
// writes, read-only while serving.
func (s *server) kioskExempt(path string) {
	if s.kioskPaths == nil { // lazily init so every server constructor is safe
		s.kioskPaths = map[string]bool{}
	}
	s.kioskPaths[path] = true
}

// kioskExemptPrefix is kioskExempt for wildcard routes. Keep prefixes tight:
// "/api/" here would open the whole admin API to uncookied loopback calls.
func (s *server) kioskExemptPrefix(prefix string) {
	s.kioskPrefixes = append(s.kioskPrefixes, prefix)
}

func (s *server) kioskPath(path string) bool {
	if s.kioskPaths[path] {
		return true
	}
	for _, prefix := range s.kioskPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// requireAuth gates /api/* and the kiosk paths behind a session cookie, only
// when a password is set. Loopback callers reach the kiosk paths uncookied.
func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kiosk := s.kioskPath(r.URL.Path)
		if !gatedPath(r.URL.Path) && !kiosk {
			next.ServeHTTP(w, r)
			return
		}
		hash := ""
		if s.store != nil {
			hash = s.store.PasswordHash()
		}
		if hash == "" {
			next.ServeHTTP(w, r)
			return
		}
		if kiosk && isLoopback(r.RemoteAddr) {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(sessionCookieName)
		if err != nil || !s.auth.Verify(hash, c.Value) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) sessionCookie(value string, maxAge int) http.Cookie {
	return http.Cookie{ //nolint:gosec // Secure omitted: plain-HTTP LAN device, a Secure cookie would be dropped
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

// LoginRequest is the body of POST /api/auth/login.
type LoginRequest struct {
	Password string `json:"password"`
}

// AuthStatusResponse reports whether auth is required and the caller logged in.
type AuthStatusResponse struct {
	Required      bool `json:"required" doc:"true if an admin password is set"`
	Authenticated bool `json:"authenticated" doc:"true if the request carries a valid session"`
}

// SetPasswordRequest is the body of POST /api/auth/password.
type SetPasswordRequest struct {
	Current string `json:"current,omitempty" doc:"Current password; required when one is already set"`
	New     string `json:"new" maxLength:"72" doc:"New password; empty disables protection"`
}

type loginInput struct{ Body LoginRequest }
type setPasswordInput struct{ Body SetPasswordRequest }
type sessionInput struct {
	Session http.Cookie `cookie:"pf_session"`
}
type setCookieOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}
type authStatusOutput struct{ Body AuthStatusResponse }

func (s *server) registerAuthRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "auth-login",
		Method:        http.MethodPost,
		Path:          "/api/auth/login",
		Summary:       "Log in to the admin UI",
		DefaultStatus: http.StatusNoContent,
		MaxBodyBytes:  4 * 1024,
	}, func(_ context.Context, in *loginInput) (*setCookieOutput, error) {
		hash := s.store.PasswordHash()
		if hash == "" || !s.checkPasswordGated(hash, in.Body.Password) {
			return nil, huma.Error401Unauthorized("invalid password")
		}
		return &setCookieOutput{SetCookie: s.issue(hash)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "auth-logout",
		Method:        http.MethodPost,
		Path:          "/api/auth/logout",
		Summary:       "Log out of the admin UI",
		DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, _ *struct{}) (*setCookieOutput, error) {
		return &setCookieOutput{SetCookie: s.sessionCookie("", -1)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-status",
		Method:      http.MethodGet,
		Path:        "/api/auth/status",
		Summary:     "Report whether auth is required and the caller is logged in",
	}, func(_ context.Context, in *sessionInput) (*authStatusOutput, error) {
		hash := s.store.PasswordHash()
		out := &authStatusOutput{}
		out.Body.Required = hash != ""
		out.Body.Authenticated = hash != "" && s.auth.Verify(hash, in.Session.Value)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "auth-set-password",
		Method:        http.MethodPost,
		Path:          "/api/auth/password",
		Summary:       "Set, change, or clear the admin password",
		DefaultStatus: http.StatusNoContent,
		MaxBodyBytes:  4 * 1024,
	}, func(_ context.Context, in *setPasswordInput) (*setCookieOutput, error) {
		cur := s.store.PasswordHash()

		// bcrypt is slow; verify and hash outside the store lock.
		if cur != "" && !s.checkPasswordGated(cur, in.Body.Current) {
			return nil, huma.Error403Forbidden("current password is incorrect")
		}
		newHash := ""
		if in.Body.New != "" {
			h, err := s.hashPasswordGated(in.Body.New)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
			newHash = h
		}

		if err := s.store.Update(func(c *config.Config) error {
			c.Auth.PasswordHash = newHash
			return nil
		}); err != nil {
			return nil, huma.Error500InternalServerError("failed to save config: " + err.Error())
		}

		// Re-issue so the caller stays logged in; prior sessions die with the old hash.
		if newHash == "" {
			return &setCookieOutput{SetCookie: s.sessionCookie("", -1)}, nil
		}
		return &setCookieOutput{SetCookie: s.issue(newHash)}, nil
	})
}

func (s *server) issue(hash string) http.Cookie {
	return s.sessionCookie(s.auth.Issue(hash), int(s.auth.TTL.Seconds()))
}
