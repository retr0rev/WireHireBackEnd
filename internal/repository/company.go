package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"jobapps/internal/models"
)

type ClientRepo struct {
	db *sql.DB
}

func NewClientRepo(db *sql.DB) *ClientRepo {
	return &ClientRepo{db: db}
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

const clientColumns = `id, c_email, c_password, phone_number,
		verified, verify_token_hash, verify_token_expiry,
		reset_token_hash, reset_token_expiry,
		company_name, company_website, company_logo_url, company_bio,
		created_by_admin_id`

func scanClient(row scanner) (*models.Client, error) {
	c := &models.Client{}
	err := row.Scan(
		&c.ID, &c.Email, &c.Password, &c.PhoneNumber,
		&c.Verified, &c.VerifyTokenHash, &c.VerifyTokenExpiry,
		&c.ResetTokenHash, &c.ResetTokenExpiry,
		&c.CompanyName, &c.CompanyWebsite, &c.CompanyLogoURL, &c.CompanyBio,
		&c.CreatedByAdminID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan client: %w", err)
	}
	return c, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (r *ClientRepo) Create(email, password, phone, verifyToken, verifyExpiry, companyName string) (*models.Client, error) {
	var phoneArg interface{}
	var phonePtr *string
	if phone != "" {
		phoneArg = phone
		phonePtr = &phone
	} else {
		phoneArg = nil
	}

	tokenHash := HashToken(verifyToken)

	res, err := r.db.Exec(
		`INSERT INTO CLIENTS
			(c_email, c_password, phone_number, verified, verify_token_hash, verify_token_expiry, company_name)
		 VALUES (?, ?, ?, 0, ?, ?, ?)`,
		email, password, phoneArg, tokenHash, verifyExpiry, companyName,
	)
	if err != nil {
		return nil, fmt.Errorf("insert client: %w", err)
	}

	id, _ := res.LastInsertId()

	return &models.Client{
		ID:                id,
		Email:             email,
		Password:          password,
		PhoneNumber:       phonePtr,
		Verified:          0,
		VerifyTokenHash:   &tokenHash,
		VerifyTokenExpiry: &verifyExpiry,
		CompanyName:       companyName,
	}, nil
}

// CreateByAdmin creates a fully-verified client on behalf of an admin. No
// verify/reset tokens are stored.
func (r *ClientRepo) CreateByAdmin(email, password, phone, companyName, companyWebsite, companyLogoURL, companyBio string, createdByAdminID int64) (*models.Client, error) {
	var phoneArg interface{}
	var phonePtr *string
	if phone != "" {
		phoneArg = phone
		phonePtr = &phone
	}

	res, err := r.db.Exec(
		`INSERT INTO CLIENTS
			(c_email, c_password, phone_number, verified, company_name, company_website, company_logo_url, company_bio, created_by_admin_id)
		 VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?)`,
		email, password, phoneArg, companyName, companyWebsite, companyLogoURL, companyBio, createdByAdminID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert client (admin): %w", err)
	}
	id, _ := res.LastInsertId()
	return &models.Client{
		ID:               id,
		Email:            email,
		Password:         password,
		PhoneNumber:      phonePtr,
		Verified:         1,
		CompanyName:      companyName,
		CompanyWebsite:   companyWebsite,
		CompanyLogoURL:   companyLogoURL,
		CompanyBio:       companyBio,
		CreatedByAdminID: &createdByAdminID,
	}, nil
}

func (r *ClientRepo) FindByEmail(email string) (*models.Client, error) {
	row := r.db.QueryRow(
		`SELECT `+clientColumns+` FROM CLIENTS WHERE c_email = ?`,
		email,
	)
	return scanClient(row)
}

func (r *ClientRepo) FindByID(id int64) (*models.Client, error) {
	row := r.db.QueryRow(
		`SELECT `+clientColumns+` FROM CLIENTS WHERE id = ?`,
		id,
	)
	return scanClient(row)
}

func (r *ClientRepo) SetVerified(id int64) error {
	_, err := r.db.Exec(
		"UPDATE CLIENTS SET verified = 1, verify_token_hash = NULL, verify_token_expiry = NULL WHERE id = ?",
		id,
	)
	return err
}

func (r *ClientRepo) FindByVerifyToken(token string) (*models.Client, error) {
	tokenHash := HashToken(token)
	row := r.db.QueryRow(
		`SELECT `+clientColumns+` FROM CLIENTS WHERE verify_token_hash = ?`,
		tokenHash,
	)
	return scanClient(row)
}

func (r *ClientRepo) SetResetToken(email, token, expiry string) error {
	tokenHash := HashToken(token)
	_, err := r.db.Exec(
		"UPDATE CLIENTS SET reset_token_hash = ?, reset_token_expiry = ? WHERE c_email = ?",
		tokenHash, expiry, email,
	)
	return err
}

func (r *ClientRepo) FindByResetToken(token string) (*models.Client, error) {
	tokenHash := HashToken(token)
	row := r.db.QueryRow(
		`SELECT `+clientColumns+` FROM CLIENTS WHERE reset_token_hash = ?`,
		tokenHash,
	)
	return scanClient(row)
}

func (r *ClientRepo) UpdatePassword(id int64, hash string) error {
	_, err := r.db.Exec(
		"UPDATE CLIENTS SET c_password = ?, reset_token_hash = NULL, reset_token_expiry = NULL WHERE id = ?",
		hash, id,
	)
	return err
}

// ListAll returns every client with job counts.
func (r *ClientRepo) ListAll() ([]models.Client, error) {
	rows, err := r.db.Query(
		`SELECT ` + clientColumns + `,
			COALESCE((SELECT COUNT(*) FROM JOBSAPPS WHERE client_id = CLIENTS.id), 0) AS jobs_total,
			COALESCE((SELECT COUNT(*) FROM JOBSAPPS WHERE client_id = CLIENTS.id AND status = 'approved'), 0) AS jobs_approved
		 FROM CLIENTS ORDER BY company_name ASC, c_email ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	defer rows.Close()

	var out []models.Client
	for rows.Next() {
		var c models.Client
		if err := rows.Scan(
			&c.ID, &c.Email, &c.Password, &c.PhoneNumber,
			&c.Verified, &c.VerifyTokenHash, &c.VerifyTokenExpiry,
			&c.ResetTokenHash, &c.ResetTokenExpiry,
			&c.CompanyName, &c.CompanyWebsite, &c.CompanyLogoURL, &c.CompanyBio,
			&c.CreatedByAdminID,
			&c.JobsTotal, &c.JobsApproved,
		); err != nil {
			return nil, fmt.Errorf("scan client row: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListPending returns all unverified employers (verified = 0).
func (r *ClientRepo) ListPending() ([]models.Client, error) {
	rows, err := r.db.Query(
		`SELECT ` + clientColumns + ` FROM CLIENTS WHERE verified = 0 ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending clients: %w", err)
	}
	defer rows.Close()

	var out []models.Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// ListPublicEmployers returns only verified employers with at least one approved
// job, suitable for the public homepage. Sensitive fields are stripped.
func (r *ClientRepo) ListPublicEmployers() ([]models.Client, error) {
	rows, err := r.db.Query(
		`SELECT id, company_name, company_website, company_logo_url, company_bio,
			COALESCE((SELECT COUNT(*) FROM JOBSAPPS WHERE client_id = CLIENTS.id AND status = 'approved'), 0) AS jobs_approved
		 FROM CLIENTS
		 WHERE verified = 1
		   AND company_name != ''
		   AND EXISTS (SELECT 1 FROM JOBSAPPS j WHERE j.client_id = CLIENTS.id AND j.status = 'approved')
		 ORDER BY company_name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list public employers: %w", err)
	}
	defer rows.Close()

	var out []models.Client
	for rows.Next() {
		var c models.Client
		if err := rows.Scan(
			&c.ID, &c.CompanyName, &c.CompanyWebsite, &c.CompanyLogoURL, &c.CompanyBio,
			&c.JobsApproved,
		); err != nil {
			return nil, fmt.Errorf("scan public employer: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type ClientUpdate struct {
	CompanyName    *string
	CompanyWebsite *string
	CompanyLogoURL *string
	CompanyBio     *string
	Phone          *string
	Email          *string
	Password       *string
}

func (r *ClientRepo) Update(id int64, u ClientUpdate) error {
	sets := []string{}
	args := []interface{}{}
	if u.CompanyName != nil {
		sets = append(sets, "company_name = ?")
		args = append(args, *u.CompanyName)
	}
	if u.CompanyWebsite != nil {
		sets = append(sets, "company_website = ?")
		args = append(args, *u.CompanyWebsite)
	}
	if u.CompanyLogoURL != nil {
		sets = append(sets, "company_logo_url = ?")
		args = append(args, *u.CompanyLogoURL)
	}
	if u.CompanyBio != nil {
		sets = append(sets, "company_bio = ?")
		args = append(args, *u.CompanyBio)
	}
	if u.Phone != nil {
		sets = append(sets, "phone_number = ?")
		if *u.Phone == "" {
			args = append(args, nil)
		} else {
			args = append(args, *u.Phone)
		}
	}
	if u.Email != nil {
		sets = append(sets, "c_email = ?")
		args = append(args, *u.Email)
	}
	if u.Password != nil {
		sets = append(sets, "c_password = ?")
		args = append(args, *u.Password)
	}
	if len(sets) == 0 {
		return nil
	}
	q := "UPDATE CLIENTS SET "
	for i, s := range sets {
		if i > 0 {
			q += ", "
		}
		q += s
	}
	q += " WHERE id = ?"
	args = append(args, id)
	_, err := r.db.Exec(q, args...)
	return err
}

func (r *ClientRepo) Delete(id int64) error {
	// Jobs cascade via app logic; do it in a tx so we don't half-delete.
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM JOBSAPPS WHERE client_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM CLIENTS WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}
