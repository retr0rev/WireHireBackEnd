package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"jobapps/internal/email"
	"jobapps/internal/handlers"
	authmw "jobapps/internal/middleware"
	"jobapps/internal/repository"
	dbpkg "jobapps/pkg/database"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

func main() {
	authmw.CheckJWTSecret()

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./database/database.db"
	}

	db, err := dbpkg.NewDB(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if len(os.Args) >= 2 && os.Args[1] == "seed-admin" {
		runSeed(db)
		return
	}

	var emailSender email.Sender
	switch {
	case os.Getenv("RESEND_API_KEY") != "":
		rs, err := email.NewResendSender()
		if err != nil {
			log.Fatalf("email: %v", err)
		}
		emailSender = rs
		log.Println("email: using Resend")
	case os.Getenv("SMTP_HOST") != "":
		emailSender = email.NewSMTPSender()
		log.Println("email: using SMTP")
	default:
		emailSender = email.NewConsoleSender()
		log.Println("email: using console (dev mode — set RESEND_API_KEY or SMTP_HOST to send real emails)")
	}

	clientRepo := repository.NewClientRepo(db)
	adminRepo := repository.NewAdminRepo(db)
	jobRepo := repository.NewJobRepo(db)

	companyHandler := handlers.NewCompanyHandler(clientRepo, emailSender)
	verifyHandler := handlers.NewVerifyHandler(clientRepo, emailSender)
	adminHandler := handlers.NewAdminHandler(adminRepo, jobRepo, clientRepo)
	jobHandler := handlers.NewJobHandler(jobRepo)

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		log.Println("WARNING: CORS_ORIGIN not set, defaulting to '*' (insecure for production). Set CORS_ORIGIN to a specific origin.")
		corsOrigin = "*"
	} else if corsOrigin == "*" {
		log.Println("WARNING: CORS_ORIGIN is set to '*'. This disables credentials and is not recommended for production.")
	}

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(authmw.CORS(corsOrigin))
	r.Use(authmw.SecurityHeaders)

	authLimiter := authmw.NewIPRateLimiter(rate.Limit(0.2), 5)
	adminAuthLimiter := authmw.NewIPRateLimiter(rate.Limit(0.1), 3)
	passwordResetLimiter := authmw.NewIPRateLimiter(rate.Limit(0.1), 3)
	publicLimiter := authmw.NewIPRateLimiter(rate.Limit(1), 10)    // public read endpoints
	writeLimiter := authmw.NewIPRateLimiter(rate.Limit(0.5), 5)    // authenticated write endpoints

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/auth", func(r chi.Router) {
		r.With(authmw.RateLimit(authLimiter)).Post("/signup", companyHandler.Signup)
		r.With(authmw.RateLimit(authLimiter)).Post("/login", companyHandler.Login)
		r.With(authmw.RateLimit(authLimiter)).Get("/verify", verifyHandler.VerifyEmail)
		r.With(authmw.RateLimit(passwordResetLimiter)).Post("/forgot-password", verifyHandler.ForgotPassword)
		r.With(authmw.RateLimit(passwordResetLimiter)).Post("/reset-password", verifyHandler.ResetPassword)
	})

	r.Route("/api/jobs", func(r chi.Router) {
		r.Use(authmw.ClientAuth)
		r.With(authmw.RateLimit(writeLimiter)).Get("/", jobHandler.List)
		r.With(authmw.RateLimit(writeLimiter)).Post("/", jobHandler.Create)
		r.Get("/{id}", jobHandler.Get)
		r.With(authmw.RateLimit(writeLimiter)).Put("/{id}", jobHandler.Update)
		r.With(authmw.RateLimit(writeLimiter)).Delete("/{id}", jobHandler.Delete)
	})

	// Authenticated client self-service profile (name, website, bio, phone).
	r.Route("/api/auth/me", func(r chi.Router) {
		r.Use(authmw.ClientAuth)
		r.Get("/", companyHandler.Me)
		r.Patch("/", companyHandler.UpdateMe)
	})

	// Client logout (clears httpOnly cookie).
	r.With(authmw.ClientAuth).Post("/api/auth/logout", companyHandler.Logout)

	r.Route("/api/admin", func(r chi.Router) {
		r.With(authmw.RateLimit(adminAuthLimiter)).Post("/login", adminHandler.Login)

		// Routes that any signed-in admin (super OR moderator) can hit.
		r.Group(func(r chi.Router) {
			r.With(authmw.AdminAuth).Get("/me", adminHandler.Me)
			r.With(authmw.AdminAuth).Patch("/me/password", adminHandler.ChangePassword)
			r.Get("/jobs", adminHandler.ListJobs)
			r.With(authmw.RateLimit(writeLimiter)).Put("/jobs/{id}/status", adminHandler.UpdateStatus)
			r.With(authmw.RateLimit(writeLimiter)).Delete("/jobs/{id}", adminHandler.DeleteJob)
			r.Post("/logout", adminHandler.Logout)
			r.Get("/employers/pending", adminHandler.ListPendingEmployers)
			r.With(authmw.RateLimit(writeLimiter)).Put("/employers/{id}/verify", adminHandler.VerifyEmployer)
		})

		// Super-admin-only routes.
		r.Group(func(r chi.Router) {
			r.Use(authmw.AdminAuth)
			r.Use(authmw.SuperAdminOnly)
			r.Get("/employers", adminHandler.ListEmployers)
			r.Get("/employers/{id}", adminHandler.GetEmployer)
			r.With(authmw.RateLimit(writeLimiter)).Post("/employers", adminHandler.CreateEmployer)
			r.With(authmw.RateLimit(writeLimiter)).Patch("/employers/{id}", adminHandler.UpdateEmployer)
			r.With(authmw.RateLimit(writeLimiter)).Delete("/employers/{id}", adminHandler.DeleteEmployer)
			r.Get("/admins", adminHandler.ListAdmins)
			r.With(authmw.RateLimit(writeLimiter)).Post("/admins", adminHandler.CreateAdmin)
		})
	})

	r.With(authmw.RateLimit(publicLimiter)).Get("/api/public/jobs", jobHandler.ListApproved)
	r.With(authmw.RateLimit(publicLimiter)).Get("/api/public/employers", companyHandler.ListPublicEmployers)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("server starting on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("server stopped")
}

func runSeed(db *sql.DB) {
	cmd := flag.NewFlagSet("seed-admin", flag.ExitOnError)
	emailFlag := cmd.String("email", "", "admin email")
	passwordFlag := cmd.String("password", "", "admin password")
	cmd.Parse(os.Args[2:])

	if *emailFlag == "" || *passwordFlag == "" {
		log.Fatal("usage: go run ./cmd/server seed-admin --email=admin@site.com --password=securepass")
	}

	email := strings.ToLower(strings.TrimSpace(*emailFlag))

	hash, err := bcrypt.GenerateFromPassword([]byte(*passwordFlag), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}

	_, err = db.Exec("INSERT INTO ADMINS (email, password) VALUES (?, ?)", email, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			log.Fatalf("admin already exists: %s (use a different email or remove the existing admin)", email)
		}
		log.Fatalf("seed: %v", err)
	}
	log.Printf("admin seeded: %s", email)
}
