package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleUser       UserRole = "user"
	RoleAdmin      UserRole = "admin"
	RoleSuperAdmin UserRole = "super_admin"
)

type UserStatus string

const (
	StatusActive              UserStatus = "active"
	StatusSuspended           UserStatus = "suspended"
	StatusPendingVerification UserStatus = "pending_verification"
)

type User struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	Email         string       `json:"email" db:"email"`
	PasswordHash  string       `json:"-" db:"password_hash"`
	FullName      string       `json:"full_name" db:"full_name"`
	CompanyName   sql.NullString `json:"company_name" db:"company_name"`
	Role          UserRole     `json:"role" db:"role"`
	Status        UserStatus   `json:"status" db:"status"`
	EmailVerified bool         `json:"email_verified" db:"email_verified"`
	VerifyToken   sql.NullString `json:"-" db:"verify_token"`
	CurrentPlanID uuid.NullUUID  `json:"current_plan_id" db:"current_plan_id"`
	LastLoginAt   sql.NullTime   `json:"last_login_at" db:"last_login_at"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`

	// Joined fields
	CompanyNameStr string `json:"company_name_str,omitempty"`
	PlanName       string `json:"plan_name,omitempty"`
	PlanSlug       string `json:"plan_slug,omitempty"`
}
