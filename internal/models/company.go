package models

type Client struct {
	ID          int64   `json:"id"`
	Email       string  `json:"email"`
	Password    string  `json:"-"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	Verified    int64   `json:"verified"`
	// Email verification / password reset internals
	VerifyTokenHash   *string `json:"-"`
	VerifyTokenExpiry *string `json:"-"`
	ResetTokenHash    *string `json:"-"`
	ResetTokenExpiry  *string `json:"-"`
	// Employer / company profile
	CompanyName       string `json:"company_name"`
	CompanyWebsite    string `json:"company_website"`
	CompanyLogoURL    string `json:"company_logo_url"`
	CompanyBio        string `json:"company_bio"`
	CreatedByAdminID  *int64 `json:"created_by_admin_id,omitempty"`
	// Counts (only populated on list endpoints)
	JobsTotal    int64 `json:"jobs_total,omitempty"`
	JobsApproved int64 `json:"jobs_approved,omitempty"`
}

type SignupRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	Phone          string `json:"phone,omitempty"`
	CompanyName    string `json:"company_name"`
	CompanyWebsite string `json:"company_website,omitempty"`
	CompanyLogoURL string `json:"company_logo_url,omitempty"`
	CompanyBio     string `json:"company_bio,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token       string `json:"token"`
	Email       string `json:"email"`
	CompanyName string `json:"company_name"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

// CreateEmployerRequest is what a Super Admin uses to create an employer
// account directly. The account is auto-verified.
type CreateEmployerRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	Phone          string `json:"phone,omitempty"`
	CompanyName    string `json:"company_name"`
	CompanyWebsite string `json:"company_website,omitempty"`
	CompanyLogoURL string `json:"company_logo_url,omitempty"`
	CompanyBio     string `json:"company_bio,omitempty"`
}

type UpdateEmployerRequest struct {
	CompanyName    *string `json:"company_name,omitempty"`
	CompanyWebsite *string `json:"company_website,omitempty"`
	CompanyLogoURL *string `json:"company_logo_url,omitempty"`
	CompanyBio     *string `json:"company_bio,omitempty"`
	Phone          *string `json:"phone,omitempty"`
	Email          *string `json:"email,omitempty"`
}

type EmployerCreateResponse struct {
	Client    Client `json:"client"`
	Temporary bool   `json:"temporary"`
}
