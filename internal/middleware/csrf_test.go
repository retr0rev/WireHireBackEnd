package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})
}

func TestCSRFMiddleware_GET_Allowed(t *testing.T) {
	handler := CSRFMiddleware(testOKHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", rec.Code)
	}
}

func TestCSRFMiddleware_POST_WithoutToken(t *testing.T) {
	handler := CSRFMiddleware(testOKHandler())
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestCSRFMiddleware_POST_WithMatchingToken(t *testing.T) {
	handler := CSRFMiddleware(testOKHandler())
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", "abc123")
	req.AddCookie(&http.Cookie{Name: CSRFCookieKey, Value: "abc123"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCSRFMiddleware_POST_WithMismatchedToken(t *testing.T) {
	handler := CSRFMiddleware(testOKHandler())
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", "abc123")
	req.AddCookie(&http.Cookie{Name: CSRFCookieKey, Value: "def456"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}
