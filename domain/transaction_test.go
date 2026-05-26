package domain

import (
	"testing"
	"time"
)

func TestTransaction_Validate(t *testing.T) {
	validTransaction := &Transaction{
		ID:              "c8c66e92-d6c5-4ad8-a89c-a1112b36bbf4",
		FromAccountID:   "a5c66e92-d6c5-4ad8-a89c-a1112b36bbf4",
		ToAccountID:     "d9c66e92-d6c5-4ad8-a89c-a1112b36bbf4",
		Amount:          15000, // 150.00
		TransactionType: "transfer",
		Status:          "initiated",
		Description:     "Rent payment",
		CreatedAt:       time.Now(),
	}

	t.Run("valid transaction", func(t *testing.T) {
		if err := validTransaction.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		invalidTx := *validTransaction
		invalidTx.ID = "invalid-uuid"
		if err := invalidTx.Validate(); err == nil {
			t.Error("expected error for invalid ID, got nil")
		}
	})

	t.Run("invalid from_account_id", func(t *testing.T) {
		invalidTx := *validTransaction
		invalidTx.FromAccountID = "invalid-uuid"
		if err := invalidTx.Validate(); err == nil {
			t.Error("expected error for invalid FromAccountID, got nil")
		}
	})

	t.Run("invalid to_account_id", func(t *testing.T) {
		invalidTx := *validTransaction
		invalidTx.ToAccountID = "invalid-uuid"
		if err := invalidTx.Validate(); err == nil {
			t.Error("expected error for invalid ToAccountID, got nil")
		}
	})

	t.Run("amount is zero", func(t *testing.T) {
		invalidTx := *validTransaction
		invalidTx.Amount = 0
		if err := invalidTx.Validate(); err == nil {
			t.Error("expected error for zero amount, got nil")
		}
	})

	t.Run("amount is negative", func(t *testing.T) {
		invalidTx := *validTransaction
		invalidTx.Amount = -100
		if err := invalidTx.Validate(); err == nil {
			t.Error("expected error for negative amount, got nil")
		}
	})

	t.Run("invalid transaction type", func(t *testing.T) {
		invalidTx := *validTransaction
		invalidTx.TransactionType = "loan"
		if err := invalidTx.Validate(); err == nil {
			t.Error("expected error for invalid transaction type, got nil")
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		invalidTx := *validTransaction
		invalidTx.Status = "completed"
		if err := invalidTx.Validate(); err == nil {
			t.Error("expected error for invalid status, got nil")
		}
	})

	t.Run("description too long", func(t *testing.T) {
		invalidTx := *validTransaction
		// Generates a 260 character string
		invalidTx.Description = "This description is exactly two hundred and sixty characters long to verify that the validation tags correctly enforce the maximum character limit for transaction descriptions in our application. If it passes validation, then something is wrong with our code."
		if len(invalidTx.Description) <= 255 {
			t.Fatalf("setup error: description must be > 255 chars, got %d", len(invalidTx.Description))
		}
		if err := invalidTx.Validate(); err == nil {
			t.Error("expected error for description too long, got nil")
		}
	})
}
