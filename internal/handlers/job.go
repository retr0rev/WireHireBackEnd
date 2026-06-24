package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"jobapps/internal/middleware"
	"jobapps/internal/models"
	"jobapps/internal/repository"

	"github.com/go-chi/chi/v5"
)

type JobHandler struct {
	jobRepo *repository.JobRepo
}

func NewJobHandler(jobRepo *repository.JobRepo) *JobHandler {
	return &JobHandler{jobRepo: jobRepo}
}

func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	clientID := middleware.GetClientID(r)

	jobs, err := h.jobRepo.ListByClient(clientID)
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

func (h *JobHandler) ListApproved(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.jobRepo.ListApproved()
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

func (h *JobHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10240)

	clientID := middleware.GetClientID(r)

	var req models.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.JobTitle) == "" {
		http.Error(w, `{"error":"job_title is required"}`, http.StatusBadRequest)
		return
	}

	if errMsg := middleware.ValidateJobTitle(req.JobTitle); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidateDescription(req.Description); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidateCategory(req.Category); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidateCategoryInList(req.Category); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}
	if errMsg := middleware.ValidateLocation(req.Location); errMsg != "" {
		http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
		return
	}

	req.JobTitle = strings.TrimSpace(req.JobTitle)
	req.Description = strings.TrimSpace(req.Description)
	req.Category = strings.TrimSpace(req.Category)
	req.Location = strings.TrimSpace(req.Location)

	job, err := h.jobRepo.Create(clientID, req.JobTitle, req.Description, req.Category, req.Location, req.BannerImageURL)
	if err != nil {
		http.Error(w, `{"error":"failed to create job"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	clientID := middleware.GetClientID(r)
	jobID, err := parseIDParam(r)
	if err != nil {
		http.Error(w, `{"error":"invalid job id"}`, http.StatusBadRequest)
		return
	}

	job, err := h.jobRepo.FindByID(jobID, clientID)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch job"}`, http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, `{"error":"job not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (h *JobHandler) Update(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10240)

	clientID := middleware.GetClientID(r)
	jobID, err := parseIDParam(r)
	if err != nil {
		http.Error(w, `{"error":"invalid job id"}`, http.StatusBadRequest)
		return
	}

	existing, err := h.jobRepo.FindByID(jobID, clientID)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch job"}`, http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, `{"error":"job not found"}`, http.StatusNotFound)
		return
	}

	var req models.UpdateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	title := existing.JobTitle
	desc := existing.Description
	cat := existing.Category
	loc := existing.Location
	bannerURL := existing.BannerImageURL
	if req.JobTitle != "" {
		if errMsg := middleware.ValidateJobTitle(req.JobTitle); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
		title = strings.TrimSpace(req.JobTitle)
	}
	if req.Description != "" {
		if errMsg := middleware.ValidateDescription(req.Description); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
		desc = strings.TrimSpace(req.Description)
	}
	if req.Category != "" {
		if errMsg := middleware.ValidateCategory(req.Category); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
		if errMsg := middleware.ValidateCategoryInList(req.Category); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
		cat = req.Category
	}
	if req.Location != "" {
		if errMsg := middleware.ValidateLocation(req.Location); errMsg != "" {
			http.Error(w, `{"error":"`+errMsg+`"}`, http.StatusBadRequest)
			return
		}
		loc = req.Location
	}
	if req.BannerImageURL != "" {
		bannerURL = req.BannerImageURL
	}

	job, err := h.jobRepo.Update(jobID, clientID, title, desc, cat, loc, bannerURL)
	if err != nil {
		http.Error(w, `{"error":"failed to update job"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (h *JobHandler) Delete(w http.ResponseWriter, r *http.Request) {
	clientID := middleware.GetClientID(r)
	jobID, err := parseIDParam(r)
	if err != nil {
		http.Error(w, `{"error":"invalid job id"}`, http.StatusBadRequest)
		return
	}

	if err := h.jobRepo.Delete(jobID, clientID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"job not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to delete job"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseIDParam(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return parseInt64(idStr)
}

func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty id")
	}
	var id int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-numeric id: %q", s)
		}
		id = id*10 + int64(c-'0')
	}
	return id, nil
}
