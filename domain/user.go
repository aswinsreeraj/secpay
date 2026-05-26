package domain

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// User represents a user of the financial application.
type User struct {
	ID        string    `json:"id" validate:"required,uuid" gorm:"primaryKey"`
	Name      string    `json:"name" validate:"required,min=2,max=100"`
	Email     string    `json:"email" validate:"required,email" gorm:"uniqueIndex"`
	Password  string    `json:"password" validate:"required,min=8"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
	UpdatedAt time.Time `json:"updated_at" validate:"required"`
}

// Validate checks if the User struct satisfies all validation rules.
func (u *User) Validate() error {
	return validate.Struct(u)
}

// UserRepository defines the database contract for User entity operations.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
}
