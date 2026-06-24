package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-thats-at-least-32-characters-long!!")
	os.Exit(m.Run())
}

func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})
}

func TestRequireAuth_NoCredentials(t *testing.T) {
	handler := requireAuth("admin", "admin_id", AdminIDKey)(testHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuth_ValidBearer(t *testing.T) {
	token, err := GenerateToken(jwt.MapClaims{"role": "admin", "admin_id": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	handler := requireAuth("admin", "admin_id", AdminIDKey)(testHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireAuth_ValidCookie(t *testing.T) {
	token, err := GenerateToken(jwt.MapClaims{"role": "admin", "admin_id": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	handler := requireAuth("admin", "admin_id", AdminIDKey)(testHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieTokenKey, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireAuth_WrongRole(t *testing.T) {
	token, err := GenerateToken(jwt.MapClaims{"role": "client", "client_id": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	handler := requireAuth("admin", "admin_id", AdminIDKey)(testHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	// Craft a token with an expired exp claim directly, since
	// GenerateToken always sets exp to 72h from now.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role":     "admin",
		"admin_id": float64(1),
		"exp":      time.Now().Add(-1 * time.Hour).Unix(),
	})
	tokenStr, err := token.SignedString(getJWTSecret())
	if err != nil {
		t.Fatal(err)
	}
	handler := requireAuth("admin", "admin_id", AdminIDKey)(testHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
