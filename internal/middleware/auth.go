package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"jobapps/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

const (
	CookieTokenKey = "token"
)

type contextKey string

const (
	ClientIDKey  contextKey = "client_id"
	AdminIDKey   contextKey = "admin_id"
	AdminRoleKey contextKey = "admin_role"
)

func getJWTSecret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

func GenerateToken(claims jwt.MapClaims) (string, error) {
	claims["exp"] = time.Now().Add(72 * time.Hour).Unix()
	claims["iat"] = time.Now().Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

// getCookieDomain returns COOKIE_DOMAIN if set, otherwise "" (host-only).
// Host-only + SameSite=None + Secure works for cross-origin (Vercel→Railway).
func getCookieDomain() string {
	return os.Getenv("COOKIE_DOMAIN")
}

func isSecure() bool {
	return os.Getenv("TLS_ENABLED") == "true"
}

// SetAuthCookie writes the JWT as an httpOnly cookie on the response.
func SetAuthCookie(w http.ResponseWriter, token string) {
	c := &http.Cookie{
		Name:     CookieTokenKey,
		Value:    token,
		Path:     "/",
		Domain:   getCookieDomain(),
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteNoneMode,
		MaxAge:   72 * 60 * 60, // 72h, matching JWT expiry
	}
	if !isSecure() {
		// SameSite=None requires Secure in browsers, but on localhost
		// (secure context) this is accepted. Drop SameSite on non-TLS
		// so the cookie works cross-origin in development.
		c.SameSite = http.SameSiteDefaultMode
	}
	http.SetCookie(w, c)
}

// ClearAuthCookie unsets the auth cookie (used on logout).
func ClearAuthCookie(w http.ResponseWriter) {
	c := &http.Cookie{
		Name:     CookieTokenKey,
		Value:    "",
		Path:     "/",
		Domain:   getCookieDomain(),
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteNoneMode,
		MaxAge:   -1,
	}
	if !isSecure() {
		c.SameSite = http.SameSiteDefaultMode
	}
	http.SetCookie(w, c)
}

// requireAuth checks role + ID claim. For admins, optionally capture the
// admin_role claim too so downstream handlers can do finer-grained RBAC.
// It reads the token from the Authorization header first, then falls back
// to the "token" httpOnly cookie.
func requireAuth(requiredRole string, idClaim string, key contextKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := ""

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					tokenStr = parts[1]
				}
			}

			// Fall back to httpOnly cookie if no Bearer header.
			if tokenStr == "" {
				if c, err := r.Cookie(CookieTokenKey); err == nil && c.Value != "" {
					tokenStr = c.Value
				}
			}

			if tokenStr == "" {
				http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
				return
			}
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return getJWTSecret(), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			role, _ := claims["role"].(string)
			if role != requiredRole {
				http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
				return
			}

			idFloat, ok := claims[idClaim].(float64)
			if !ok {
				http.Error(w, `{"error":"invalid token payload"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), key, int64(idFloat))

			// Surface admin role to handlers for fine-grained checks.
			if requiredRole == "admin" {
				if roleStr, ok := claims["admin_role"].(string); ok {
					ctx = context.WithValue(ctx, AdminRoleKey, roleStr)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClientAuth(next http.Handler) http.Handler {
	return requireAuth("client", "client_id", ClientIDKey)(next)
}

func AdminAuth(next http.Handler) http.Handler {
	return requireAuth("admin", "admin_id", AdminIDKey)(next)
}

// AdminRoleAtLeast returns a middleware that allows admins whose role is in
// the given set. Must run after AdminAuth so AdminRoleKey is set.
func AdminRoleAtLeast(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(AdminRoleKey).(string)
			for _, a := range allowed {
				if role == a {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
		})
	}
}

// Convenience aliases.
var (
	SuperAdminOnly      = AdminRoleAtLeast(models.AdminRoleSuperAdmin)
	ModeratorOrAbove    = AdminRoleAtLeast(models.AdminRoleSuperAdmin, models.AdminRoleModerator)
)

func GetClientID(r *http.Request) int64 {
	id, _ := r.Context().Value(ClientIDKey).(int64)
	return id
}

func GetAdminID(r *http.Request) int64 {
	id, _ := r.Context().Value(AdminIDKey).(int64)
	return id
}

func GetAdminRole(r *http.Request) string {
	role, _ := r.Context().Value(AdminRoleKey).(string)
	return role
}
