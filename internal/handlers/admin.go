package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"jobapps/internal/middleware"
	"jobapps/internal/models"
	"jobapps/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	adminRepo *repository.AdminRepo
	jobRepo   *repository.JobRepo
	clientRepo *repository.ClientRepo
}

func NewAdminHandler(adminRepo *repository.AdminRepo, jobRepo *repository.JobRepo, clientRepo *repository.ClientRepo) *AdminHandler {
	return &AdminHandler{adminRepo: adminRepo, jobRepo: jobRepo, clientRepo: clientRepo}
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req models.AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Email = middleware.NormalizeEmail(req.Email)

	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, http.StatusBadRequest)
		return
	}

	admin, err := h.adminRepo.FindByEmail(req.Email)
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}
	if admin == nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	if admin.AdminRole == "" {
		admin.AdminRole = models.AdminRoleSuperAdmin
	}

	claims := jwt.MapClaims{
		"admin_id":   admin.ID,
		"role":       "admin",
		"admin_role": admin.AdminRole,
	}
	token, err := middleware.GenerateToken(claims)
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}

	middleware.SetAuthCookie(w, token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.AdminLoginResponse{
		Token:     token,
		Email:     admin.Email,
		AdminRole: admin.AdminRole,
	})
}

// Me returns the current admin's profile + role.
func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	id := middleware.GetAdminID(r)
	admin, err := h.adminRepo.FindByID(id)
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}
	if admin == nil {
		http.Error(w, `{"error":"admin not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(admin)
}

func (h *AdminHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.jobRepo.ListAll()
	if err != nil {
		http.Error(w, `{"error":"failed to fetch jobs"}`, http.StatusInternalServerError)
		return
	}

	if jobs == nil {
		jobs = []models.JobApp{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (h *AdminHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)

	jobID, err := parseIDParam(r)
	if err != nil {
		http.Error(w, `{"error":"invalid job id"}`, http.StatusBadRequest)
		return
	}

	var req models.UpdateJobStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Status != models.StatusApproved && req.Status != models.StatusRejected {
		http.Error(w, `{"error":"status must be 'approved' or 'rejected'"}`, http.StatusBadRequest)
		return
	}

	if err := h.jobRepo.UpdateStatus(jobID, req.Status); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"job not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to update job status"}`, http.StatusInternalServerError)
		return
	}

	job, _ := h.jobRepo.FindByIDAdmin(jobID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// DeleteJob hard-deletes any job. Available to both Super Admin and
// Moderator (uses ModeratorOrAbove guard at the route level).
func (h *AdminHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseIDParam(r)
	if err != nil {
		http.Error(w, `{"error":"invalid job id"}`, http.StatusBadRequest)
		return
	}

	if err := h.jobRepo.DeleteAdmin(jobID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"job not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to delete job"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Employer (CLIENT) management (Super Admin only) ----

func (h *AdminHandler) ListEmployers(w http.ResponseWriter, r *http.Request) {
	clients, err := h.clientRepo.ListAll()
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}
	if clients == nil {
		clients = []models.Client{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func (h *AdminHandler) GetEmployer(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid employer id"}`, http.StatusBadRequest)
		return
	}
	c, err := h.clientRepo.FindByID(id)
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, `{"error":"employer not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func (h *AdminHandler) CreateEmployer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req models.CreateEmployerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Email = middleware.NormalizeEmail(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	req.CompanyWebsite = strings.TrimSpace(req.CompanyWebsite)
	req.CompanyLogoURL = strings.TrimSpace(req.CompanyLogoURL)
	req.CompanyBio = strings.TrimSpace(req.CompanyBio)

	if !middleware.ValidateEmail(req.Email) {
		http.Error(w, `{"error":"invalid email format"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidatePassword(req.Password); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.CompanyName) == "" {
		http.Error(w, `{"error":"company_name is required"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidateOptionalText("company_website", req.CompanyWebsite, 2048); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidateOptionalText("company_logo_url", req.CompanyLogoURL, 2048); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidateOptionalText("company_bio", req.CompanyBio, 2000); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if req.Phone != "" {
		if errMsg := middleware.ValidatePhone(req.Phone); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
	}

	if existing, _ := h.clientRepo.FindByEmail(req.Email); existing != nil {
		http.Error(w, `{"error":"email already registered"}`, http.StatusConflict)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}

	adminID := middleware.GetAdminID(r)
	created, err := h.clientRepo.CreateByAdmin(
		req.Email, string(hash), req.Phone,
		req.CompanyName, req.CompanyWebsite, req.CompanyLogoURL, req.CompanyBio,
		adminID,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to create employer: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.EmployerCreateResponse{
		Client:      *created,
		Temporary:   false,
		RawPassword: req.Password,
	})
}

func (h *AdminHandler) UpdateEmployer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid employer id"}`, http.StatusBadRequest)
		return
	}

	var req models.UpdateEmployerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.CompanyName != nil && strings.TrimSpace(*req.CompanyName) == "" {
		http.Error(w, `{"error":"company_name cannot be empty"}`, http.StatusBadRequest)
		return
	}
	if req.CompanyWebsite != nil {
		if errMsg := middleware.ValidateOptionalText("company_website", *req.CompanyWebsite, 2048); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
	}
	if req.CompanyLogoURL != nil {
		if errMsg := middleware.ValidateOptionalText("company_logo_url", *req.CompanyLogoURL, 2048); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
	}
	if req.CompanyBio != nil {
		if errMsg := middleware.ValidateOptionalText("company_bio", *req.CompanyBio, 2000); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
	}
	if req.Email != nil {
		normalized := middleware.NormalizeEmail(*req.Email)
		req.Email = &normalized
	}

	if err := h.clientRepo.Update(id, repository.ClientUpdate{
		CompanyName:    req.CompanyName,
		CompanyWebsite: req.CompanyWebsite,
		CompanyLogoURL: req.CompanyLogoURL,
		CompanyBio:     req.CompanyBio,
		Phone:          req.Phone,
		Email:          req.Email,
	}); err != nil {
		http.Error(w, `{"error":"update failed: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	updated, _ := h.clientRepo.FindByID(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (h *AdminHandler) DeleteEmployer(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid employer id"}`, http.StatusBadRequest)
		return
	}
	if err := h.clientRepo.Delete(id); err != nil {
		http.Error(w, `{"error":"delete failed: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Admin management (Super Admin only) ----

func (h *AdminHandler) ListAdmins(w http.ResponseWriter, r *http.Request) {
	admins, err := h.adminRepo.ListAll()
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}
	if admins == nil {
		admins = []models.Admin{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(admins)
}

func (h *AdminHandler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req models.CreateAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	req.Email = middleware.NormalizeEmail(req.Email)

	if !middleware.ValidateEmail(req.Email) {
		http.Error(w, `{"error":"invalid email format"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidatePassword(req.Password); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		req.Role = models.AdminRoleModerator
	}
	if req.Role != models.AdminRoleSuperAdmin && req.Role != models.AdminRoleModerator {
		http.Error(w, `{"error":"role must be 'super_admin' or 'moderator'"}`, http.StatusBadRequest)
		return
	}

	admin, err := h.adminRepo.Create(req.Email, req.Password, req.Role)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			http.Error(w, `{"error":"admin already exists"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"create failed: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(admin)
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	middleware.ClearAuthCookie(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.MessageResponse{
		Message: "logged out",
	})
}

// ChangePassword lets the current admin change their own password.
func (h *AdminHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	id := middleware.GetAdminID(r)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if errMsg := middleware.ValidatePassword(req.NewPassword); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}

	admin, err := h.adminRepo.FindByID(id)
	if err != nil || admin == nil {
		http.Error(w, `{"error":"admin not found"}`, http.StatusNotFound)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, `{"error":"current password is incorrect"}`, http.StatusForbidden)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}

	if err := h.adminRepo.UpdatePassword(id, string(hash)); err != nil {
		http.Error(w, `{"error":"failed to update password"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.MessageResponse{Message: "password updated"})
}

// ---- Employer verification (Moderator+) ----

func (h *AdminHandler) ListPendingEmployers(w http.ResponseWriter, r *http.Request) {
	clients, err := h.clientRepo.ListPending()
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}
	if clients == nil {
		clients = []models.Client{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func (h *AdminHandler) VerifyEmployer(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid employer id"}`, http.StatusBadRequest)
		return
	}

	if err := h.clientRepo.SetVerified(id); err != nil {
		http.Error(w, `{"error":"failed to verify employer"}`, http.StatusInternalServerError)
		return
	}

	updated, _ := h.clientRepo.FindByID(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}
