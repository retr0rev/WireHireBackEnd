CREATE TABLE ADMINS (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    admin_role TEXT NOT NULL DEFAULT 'super_admin'
);

CREATE TABLE CLIENTS (
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
);

CREATE TABLE JOBSAPPS (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id INTEGER NOT NULL,
    jobtitle TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    category TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    banner_image_url TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (client_id) REFERENCES CLIENTS(id)
);
