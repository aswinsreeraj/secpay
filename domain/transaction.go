package domain

import (
	"context"
	"time"
)

// Transaction represents a financial transaction between accounts.
// For double-entry bookkeeping:
// - Deposits: FromAccountID is the System Account ID, ToAccountID is the User Account ID.
// - Withdrawals: FromAccountID is the User Account ID, ToAccountID is the System Account ID.
// - Transfers: Both FromAccountID and ToAccountID are valid User Account IDs.
type Transaction struct {
	ID              string    `json:"id" validate:"required,uuid" gorm:"primaryKey"`
	FromAccountID   string    `json:"from_account_id" validate:"required,uuid" gorm:"index"`
	ToAccountID     string    `json:"to_account_id" validate:"required,uuid" gorm:"index"`
	Amount          int64     `json:"amount" validate:"required,gt=0"` // Stored in minor units (e.g. cents/paise), must be > 0.
	TransactionType string    `json:"transaction_type" validate:"required,oneof=deposit withdraw transfer"`
	Status          string    `json:"status" validate:"required,oneof=pending completed failed"`
	Description     string    `json:"description" validate:"max=255"`
	CreatedAt       time.Time `json:"created_at" validate:"required"`
}

// Validate checks if the Transaction struct satisfies all validation rules.
func (t *Transaction) Validate() error {
	return validate.Struct(t)
}

// TransactionRepository defines the database contract for Transaction entity operations.
type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
	GetByID(ctx context.Context, id string) (*Transaction, error)
	GetByAccountID(ctx context.Context, accountID string) ([]*Transaction, error)
}
