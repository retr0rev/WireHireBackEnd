package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"jobapps/internal/repository"
	dbpkg "jobapps/pkg/database"
)

func TestCreateEmployer_NoRawPasswordInResponse(t *testing.T) {
	f, err := os.CreateTemp("", "wirehire-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

	db, err := dbpkg.NewDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Seed a test admin with a known hash so we can reference the admin_id.
	_, err = db.Exec(
		"INSERT INTO ADMINS (email, password, admin_role) VALUES (?, ?, ?)",
		"admin@test.com", "$2a$10$fakehashfortesting", "super_admin",
	)
	if err != nil {
		t.Fatal(err)
	}

	clientRepo := repository.NewClientRepo(db)
	adminRepo := repository.NewAdminRepo(db)
	jobRepo := repository.NewJobRepo(db)
	h := NewAdminHandler(adminRepo, jobRepo, clientRepo)

	body := `{"email":"employer@test.com","password":"ValidPass123!","company_name":"TestCo"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.CreateEmployer(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if _, ok := resp["raw_password"]; ok {
		t.Error("response contains raw_password field — plaintext password leak!")
	}

	// Verify expected response shape: client + temporary (no raw_password).
	if _, ok := resp["client"]; !ok {
		t.Error("response missing client field")
	}
	if _, ok := resp["temporary"]; !ok {
		t.Error("response missing temporary field")
	}
}
