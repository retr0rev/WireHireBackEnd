package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	CSRFCookieKey = "csrf"
)

// SetCSRFCookie sets a non-HttpOnly CSRF cookie so that JavaScript can read it
// and include the value in the X-CSRF-Token header on state-changing requests.
func SetCSRFCookie(w http.ResponseWriter, token string) {
	c := &http.Cookie{
		Name:     CSRFCookieKey,
		Value:    token,
		Path:     "/",
		Domain:   getCookieDomain(),
		HttpOnly: false, // JS must be able to read it
		Secure:   isSecure(),
		SameSite: http.SameSiteNoneMode, // cross-origin in production
		MaxAge:   86400,
	}
	if !isSecure() {
		// Dev mode (Vite proxy, same-origin): SameSite=None requires
		// Secure, which doesn't work over HTTP. Use Lax instead —
		// same-origin requests don't need None.
		c.SameSite = http.SameSiteDefaultMode
	}
	http.SetCookie(w, c)
}

// GenerateCSRFToken returns a 32-byte (64 hex char) random token.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CSRFCookieToken extracts the CSRF token from the csrf cookie.
func CSRFCookieToken(r *http.Request) string {
	if c, err := r.Cookie(CSRFCookieKey); err == nil {
		return c.Value
	}
	return ""
}

// publicAuthPaths lists public auth endpoints that don't require CSRF protection.
// These are safe because they're either:
// - Registration (signup) - no existing session to hijack
// - Login - establishes session, no prior auth state
// - Password reset - initiated via email token, not session
var publicAuthPaths = map[string]struct{}{
	"/api/auth/signup":          {},
	"/api/auth/login":           {},
	"/api/auth/forgot-password": {},
	"/api/auth/reset-password":  {},
}

// CSRFMiddleware returns a middleware that enforces double-submit CSRF
// protection on state-changing methods (POST, PUT, PATCH, DELETE).
// GET, HEAD, and OPTIONS requests are always allowed through.
// Public auth endpoints are also exempt.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip non-state-changing methods.
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		// Skip public auth endpoints that don't require CSRF.
		if _, ok := publicAuthPaths[r.URL.Path]; ok {
			next.ServeHTTP(w, r)
			return
		}

		cookieToken := CSRFCookieToken(r)
		headerToken := r.Header.Get("X-CSRF-Token")

		if cookieToken == "" || headerToken == "" || cookieToken != headerToken {
			http.Error(w, `{"error":"csrf token mismatch"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
