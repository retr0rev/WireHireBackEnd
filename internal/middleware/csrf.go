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
		SameSite: http.SameSiteNoneMode, // cross-origin (localhost:5174→:8080)
		MaxAge:   86400,
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

// CSRFMiddleware returns a middleware that enforces double-submit CSRF
// protection on state-changing methods (POST, PUT, PATCH, DELETE).
// GET, HEAD, and OPTIONS requests are always allowed through.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip non-state-changing methods.
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
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
