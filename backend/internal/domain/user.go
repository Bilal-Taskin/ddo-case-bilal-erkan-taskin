package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleMember     UserRole = "MEMBER"
	RoleReception  UserRole = "RECEPTION"
	RoleTrainer    UserRole = "TRAINER"
	RoleManager    UserRole = "MANAGER"
	RoleAccountant UserRole = "ACCOUNTANT"
)

var ValidRoles = map[UserRole]bool{
	RoleMember:     true,
	RoleReception:  true,
	RoleTrainer:    true,
	RoleManager:    true,
	RoleAccountant: true,
}

func (r UserRole) IsValid() bool {
	return ValidRoles[r]
}

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	Phone        *string    `json:"phone,omitempty"`
	Role         UserRole   `json:"role"`
	HomeBranchID *uuid.UUID `json:"home_branch_id,omitempty"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

var (
	ErrUserNotFound      = errors.New("kullanici bulunamadi")
	ErrUserAlreadyExists = errors.New("bu e-posta adresiyle kayitli bir kullanici zaten mevcut")
	ErrInvalidRole       = errors.New("gecersiz kullanici rolu")
	ErrInactiveUser      = errors.New("kullanici hesabi aktif degil")
	ErrInvalidPassword   = errors.New("gecersiz e-posta veya parola")
	ErrUnauthorized      = errors.New("oturum acilmasi gerekiyor")
	ErrForbidden         = errors.New("bu islem icin yetkiniz bulunmamaktadir")
)
