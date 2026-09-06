package models

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

type Team struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	ID            string               `json:"id"`
	TeamID        string               `json:"team_id"`
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	Description   string               `json:"description"`
	RepositoryURL string               `json:"repository_url"`
	OwnerEmail    string               `json:"owner_email"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	Environments  []ServiceEnvironment `json:"environments,omitempty"`
}

type ServiceEnvironment struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateServiceInput struct {
	TeamID        string   `json:"team_id"`
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Description   string   `json:"description"`
	RepositoryURL string   `json:"repository_url"`
	OwnerEmail    string   `json:"owner_email"`
	Environments  []string `json:"environments"`
}