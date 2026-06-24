package repository

import (
	"database/sql"
	"fmt"
	"jobapps/internal/models"
)

type JobRepo struct {
	db *sql.DB
}

func NewJobRepo(db *sql.DB) *JobRepo {
	return &JobRepo{db: db}
}

const jobSelectColumns = `j.id, j.client_id, j.jobtitle, j.description, j.status,
	j.category, j.location, c.c_email, c.phone_number,
	COALESCE(c.company_name, ''), COALESCE(c.company_website, ''),
	COALESCE(c.company_logo_url, ''), COALESCE(c.company_bio, ''),
	COALESCE(j.banner_image_url, '')`

const jobJoinClause = ` FROM JOBSAPPS j JOIN CLIENTS c ON j.client_id = c.id `

func scanJobWithContact(s interface {
	Scan(dest ...interface{}) error
}) (*models.JobApp, error) {
	j := &models.JobApp{}
	var phone sql.NullString
	err := s.Scan(
		&j.ID, &j.ClientID, &j.JobTitle, &j.Description,
		&j.Status, &j.Category, &j.Location,
		&j.ClientEmail, &phone,
		&j.CompanyName, &j.CompanyWebsite, &j.CompanyLogoURL, &j.CompanyBio,
		&j.BannerImageURL,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan job: %w", err)
	}
	if phone.Valid {
		s := phone.String
		j.PhoneNumber = &s
	}
	return j, nil
}

func (r *JobRepo) Create(clientID int64, title, description, category, location, bannerImageURL string) (*models.JobApp, error) {
	res, err := r.db.Exec(
		"INSERT INTO JOBSAPPS (client_id, jobtitle, description, status, category, location, banner_image_url) VALUES (?, ?, ?, ?, ?, ?, ?)",
		clientID, title, description, models.StatusPending, category, location, bannerImageURL,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job: %w", err)
	}

	id, _ := res.LastInsertId()
	return r.FindByIDAdmin(id)
}

func (r *JobRepo) ListByClient(clientID int64) ([]models.JobApp, error) {
	rows, err := r.db.Query(
		"SELECT "+jobSelectColumns+jobJoinClause+"WHERE j.client_id = ? ORDER BY j.id DESC",
		clientID,
	)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.JobApp
	for rows.Next() {
		j, err := scanJobWithContact(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}

func (r *JobRepo) ListAll() ([]models.JobApp, error) {
	rows, err := r.db.Query(
		"SELECT " + jobSelectColumns + jobJoinClause + "ORDER BY j.id DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("query all jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.JobApp
	for rows.Next() {
		j, err := scanJobWithContact(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}

func (r *JobRepo) ListApproved() ([]models.JobApp, error) {
	rows, err := r.db.Query(
		"SELECT "+jobSelectColumns+jobJoinClause+"WHERE j.status = ? ORDER BY j.id DESC",
		models.StatusApproved,
	)
	if err != nil {
		return nil, fmt.Errorf("query approved jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.JobApp
	for rows.Next() {
		j, err := scanJobWithContact(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}

func (r *JobRepo) FindByID(id, clientID int64) (*models.JobApp, error) {
	row := r.db.QueryRow(
		"SELECT "+jobSelectColumns+jobJoinClause+"WHERE j.id = ? AND j.client_id = ?",
		id, clientID,
	)
	return scanJobWithContact(row)
}

func (r *JobRepo) FindByIDAdmin(id int64) (*models.JobApp, error) {
	row := r.db.QueryRow(
		"SELECT "+jobSelectColumns+jobJoinClause+"WHERE j.id = ?",
		id,
	)
	return scanJobWithContact(row)
}

func (r *JobRepo) UpdateStatus(id int64, status string) error {
	res, err := r.db.Exec(
		"UPDATE JOBSAPPS SET status = ? WHERE id = ?",
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *JobRepo) Update(id, clientID int64, title, description, category, location, bannerImageURL string) (*models.JobApp, error) {
	_, err := r.db.Exec(
		"UPDATE JOBSAPPS SET jobtitle = ?, description = ?, category = ?, location = ?, banner_image_url = ? WHERE id = ? AND client_id = ?",
		title, description, category, location, bannerImageURL, id, clientID,
	)
	if err != nil {
		return nil, fmt.Errorf("update job: %w", err)
	}
	return r.FindByID(id, clientID)
}

func (r *JobRepo) Delete(id, clientID int64) error {
	res, err := r.db.Exec(
		"DELETE FROM JOBSAPPS WHERE id = ? AND client_id = ?",
		id, clientID,
	)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteAdmin removes a job without an ownership check. Used by admin
// moderation endpoints. Returns sql.ErrNoRows if the job does not exist.
func (r *JobRepo) DeleteAdmin(id int64) error {
	res, err := r.db.Exec("DELETE FROM JOBSAPPS WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete job (admin): %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
