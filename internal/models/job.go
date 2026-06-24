package models

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

type JobApp struct {
	ID          int64   `json:"id"`
	ClientID    int64   `json:"client_id"`
	JobTitle    string  `json:"job_title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Category    string  `json:"category"`
	Location    string  `json:"location"`
	ClientEmail string  `json:"client_email"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	// Embedded employer profile (denormalised for read efficiency)
	CompanyName    string `json:"company_name"`
	CompanyWebsite string `json:"company_website"`
	CompanyLogoURL string `json:"company_logo_url"`
	CompanyBio     string `json:"company_bio"`
	BannerImageURL string `json:"banner_image_url"`
}

type CreateJobRequest struct {
	JobTitle       string `json:"job_title"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	Location       string `json:"location"`
	BannerImageURL string `json:"banner_image_url,omitempty"`
}

type UpdateJobRequest struct {
	JobTitle       string `json:"job_title,omitempty"`
	Description    string `json:"description,omitempty"`
	Category       string `json:"category,omitempty"`
	Location       string `json:"location,omitempty"`
	BannerImageURL string `json:"banner_image_url,omitempty"`
}

type UpdateJobStatusRequest struct {
	Status string `json:"status"`
}
