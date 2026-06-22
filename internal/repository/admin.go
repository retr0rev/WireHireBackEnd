package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"jobapps/internal/models"

	"golang.org/x/crypto/bcrypt"
)

type AdminRepo struct {
	db *sql.DB
}

func NewAdminRepo(db *sql.DB) *AdminRepo {
	return &AdminRepo{db: db}
}

func scanAdmin(row scanner) (*models.Admin, error) {
	a := &models.Admin{}
	if err := row.Scan(&a.ID, &a.Email, &a.Password, &a.AdminRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan admin: %w", err)
	}
	return a, nil
}

func (r *AdminRepo) FindByEmail(email string) (*models.Admin, error) {
	row := r.db.QueryRow(
		"SELECT id, email, password, admin_role FROM ADMINS WHERE email = ?",
		email,
	)
	return scanAdmin(row)
}

func (r *AdminRepo) FindByID(id int64) (*models.Admin, error) {
	row := r.db.QueryRow(
		"SELECT id, email, password, admin_role FROM ADMINS WHERE id = ?",
		id,
	)
	return scanAdmin(row)
}

func (r *AdminRepo) Create(email, password, role string) (*models.Admin, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	if role == "" {
		role = models.AdminRoleSuperAdmin
	}
	res, err := r.db.Exec(
		"INSERT INTO ADMINS (email, password, admin_role) VALUES (?, ?, ?)",
		email, string(hash), role,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &models.Admin{ID: id, Email: email, Password: string(hash), AdminRole: role}, nil
}

func (r *AdminRepo) ListAll() ([]models.Admin, error) {
	rows, err := r.db.Query("SELECT id, email, password, admin_role FROM ADMINS ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Admin
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *AdminRepo) UpdatePassword(id int64, hash string) error {
	_, err := r.db.Exec("UPDATE ADMINS SET password = ? WHERE id = ?", hash, id)
	return err
}
