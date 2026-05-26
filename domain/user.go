package domain

import (
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// User represents a user of the financial application.
type User struct {
	ID        string    `json:"id" validate:"required,uuid"`
	Name      string    `json:"name" validate:"required,min=2,max=100"`
	Email     string    `json:"email" validate:"required,email"`
	Password  string    `json:"password" validate:"required,min=8"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
	UpdatedAt time.Time `json:"updated_at" validate:"required"`
}

// Validate checks if the User struct satisfies all validation rules.
func (u *User) Validate() error {
	return validate.Struct(u)
}
