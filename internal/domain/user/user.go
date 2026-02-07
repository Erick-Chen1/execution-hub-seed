package user

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Role represents a user role.
type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleOperator Role = "OPERATOR"
	RoleViewer   Role = "VIEWER"
)

// Type represents a user type.
type Type string

const (
	TypeHuman Type = "HUMAN"
	TypeAgent Type = "AGENT"
)

// Status represents user status.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusDisabled Status = "DISABLED"
)

// User represents a system user (human or agent).
type User struct {
	ID           int64      `json:"id"`
	UserID       uuid.UUID  `json:"userId"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         Role       `json:"role"`
	Type         Type       `json:"type"`
	OwnerUserID  *uuid.UUID `json:"ownerUserId,omitempty"`
	Status       Status     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

var usernamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{2,30}[A-Za-z0-9]$`)

func ValidateUsername(username string) error {
	if username == "" {
		return errors.New("username is required")
	}
	if !usernamePattern.MatchString(username) {
		return errors.New("username must be 4-32 chars, start with a letter, and contain only letters, digits, '.', '_' or '-'")
	}
	return nil
}

func ValidatePassword(password string, username string) error {
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return errors.New("password must include upper, lower, digit, and special character")
	}
	if username != "" {
		lower := strings.ToLower(password)
		if strings.Contains(lower, strings.ToLower(username)) {
			return errors.New("password must not contain username")
		}
	}
	return nil
}

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash string, password string) bool {
	if hash == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ValidateRole(role Role) error {
	switch role {
	case RoleAdmin, RoleOperator, RoleViewer:
		return nil
	default:
		return errors.New("invalid role")
	}
}

func ValidateType(t Type) error {
	switch t {
	case TypeHuman, TypeAgent:
		return nil
	default:
		return errors.New("invalid user type")
	}
}

func ValidateStatus(status Status) error {
	switch status {
	case StatusActive, StatusDisabled:
		return nil
	default:
		return errors.New("invalid status")
	}
}
