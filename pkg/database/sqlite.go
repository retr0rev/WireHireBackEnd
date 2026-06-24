package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func NewDB(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("resolve db path: %w", err)
	}

	db, err := sql.Open("sqlite", absPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db.SetMaxOpenConns(1)

	// Low-memory PRAGMAs for serverless environments
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-2000",
		"PRAGMA mmap_size=0",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("pragma %s: %w", p, err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS ADMINS (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			admin_role TEXT NOT NULL DEFAULT 'super_admin'
		)`,
		`CREATE TABLE IF NOT EXISTS CLIENTS (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			c_email TEXT NOT NULL UNIQUE,
			c_password TEXT NOT NULL,
			phone_number TEXT,
			verified INTEGER NOT NULL DEFAULT 0,
			verify_token_hash TEXT,
			verify_token_expiry DATETIME,
			reset_token_hash TEXT,
			reset_token_expiry DATETIME,
			company_name TEXT NOT NULL DEFAULT '',
			company_website TEXT NOT NULL DEFAULT '',
			company_logo_url TEXT NOT NULL DEFAULT '',
			company_bio TEXT NOT NULL DEFAULT '',
			created_by_admin_id INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS JOBSAPPS (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id INTEGER NOT NULL,
			jobtitle TEXT NOT NULL,
			description TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			category TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (client_id) REFERENCES CLIENTS(id)
		)`,
	}

	for _, s := range schema {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_jobsapps_status ON JOBSAPPS(status)`,
		`CREATE INDEX IF NOT EXISTS idx_jobsapps_client_id ON JOBSAPPS(client_id)`,
	}
	for _, s := range indexes {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}

	// Idempotent column additions for older databases that pre-date the
	// employer-profile / admin-role fields. SQLite has no "ADD COLUMN IF NOT
	// EXISTS", so we detect and ignore "duplicate column" errors.
	addCols := []struct {
		table string
		col   string
		def   string
	}{
		{"ADMINS", "admin_role", "TEXT NOT NULL DEFAULT 'super_admin'"},
		{"CLIENTS", "company_name", "TEXT NOT NULL DEFAULT ''"},
		{"CLIENTS", "company_website", "TEXT NOT NULL DEFAULT ''"},
		{"CLIENTS", "company_logo_url", "TEXT NOT NULL DEFAULT ''"},
		{"CLIENTS", "company_bio", "TEXT NOT NULL DEFAULT ''"},
		{"CLIENTS", "created_by_admin_id", "INTEGER"},
		{"JOBSAPPS", "banner_image_url", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range addCols {
		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", c.table, c.col, c.def)
		if _, err := db.Exec(stmt); err != nil {
			if !isDuplicateColumnErr(err) {
				return fmt.Errorf("alter %s add %s: %w", c.table, c.col, err)
			}
		}
	}

	return nil
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}

var _ = errors.New
