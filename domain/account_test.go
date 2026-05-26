package domain

import (
	"testing"
	"time"
)

func TestAccount_Validate(t *testing.T) {
	validAccount := &Account{
		ID:            "a5c66e92-d6c5-4ad8-a89c-a1112b36bbf4",
		UserID:        "b8c66e92-d6c5-4ad8-a89c-a1112b36bbf4",
		AccountNumber: "1234567890",
		AccountType:   "savings",
		Balance:       50000, // 500.00 USD
		Currency:      "USD",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	t.Run("valid account", func(t *testing.T) {
		if err := validAccount.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		invalidAccount := *validAccount
		invalidAccount.ID = "invalid-uuid"
		if err := invalidAccount.Validate(); err == nil {
			t.Error("expected error for invalid ID, got nil")
		}
	})

	t.Run("invalid userid", func(t *testing.T) {
		invalidAccount := *validAccount
		invalidAccount.UserID = "invalid-uuid"
		if err := invalidAccount.Validate(); err == nil {
			t.Error("expected error for invalid UserID, got nil")
		}
	})

	t.Run("invalid account number length", func(t *testing.T) {
		invalidAccount := *validAccount
		invalidAccount.AccountNumber = "12345" // 5 chars instead of 10
		if err := invalidAccount.Validate(); err == nil {
			t.Error("expected error for account number length, got nil")
		}
	})

	t.Run("invalid account number format", func(t *testing.T) {
		invalidAccount := *validAccount
		invalidAccount.AccountNumber = "123456789-" // contains hyphen
		if err := invalidAccount.Validate(); err == nil {
			t.Error("expected error for non-alphanumeric account number, got nil")
		}
	})

	t.Run("invalid account type", func(t *testing.T) {
		invalidAccount := *validAccount
		invalidAccount.AccountType = "investment" // invalid value
		if err := invalidAccount.Validate(); err == nil {
			t.Error("expected error for invalid account type, got nil")
		}
	})

	t.Run("invalid currency length", func(t *testing.T) {
		invalidAccount := *validAccount
		invalidAccount.Currency = "US" // length 2 instead of 3
		if err := invalidAccount.Validate(); err == nil {
			t.Error("expected error for invalid currency length, got nil")
		}
	})
}
