package domain

import (
	"context"
	"time"
)

// Account represents a financial account.
type Account struct {
	ID            string    `json:"id" validate:"required,uuid" gorm:"primaryKey"`
	UserID        string    `json:"user_id" validate:"required,uuid" gorm:"index"`
	AccountNumber string    `json:"account_number" validate:"required,alphanum,len=10" gorm:"uniqueIndex"`
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

// AccountRepository defines the database contract for Account entity operations.
type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id string) (*Account, error)
	GetByAccountNumber(ctx context.Context, accNum string) (*Account, error)
	Update(ctx context.Context, account *Account) error
	TransferBalance(ctx context.Context, fromAccountID, toAccountID string, amount int64, description string) error
}
