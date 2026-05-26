package domain

import (
	"time"
)

// Account represents a financial account.
type Account struct {
	ID            string    `json:"id" validate:"required,uuid"`
	UserID        string    `json:"user_id" validate:"required,uuid"`
	AccountNumber string    `json:"account_number" validate:"required,alphanum,len=10"`
	AccountType   string    `json:"account_type" validate:"required,oneof=savings checking credit"`
	Balance       int64     `json:"balance"` // Stored in minor units (e.g., cents/paise) to prevent float issues.
	Currency      string    `json:"currency" validate:"required,len=3"` // e.g., USD, EUR, INR
	CreatedAt     time.Time `json:"created_at" validate:"required"`
	UpdatedAt     time.Time `json:"updated_at" validate:"required"`
}

// Validate checks if the Account struct satisfies all validation rules.
func (a *Account) Validate() error {
	return validate.Struct(a)
}
