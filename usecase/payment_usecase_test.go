package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"secpay/domain"
)

// mockAccountRepository mock implementation
type mockAccountRepository struct {
	domain.AccountRepository
	transferErr error
}

func (m *mockAccountRepository) TransferBalance(ctx context.Context, fromAccountID, toAccountID string, amount int64, description string) error {
	return m.transferErr
}

// mockTransactionRepository mock implementation
type mockTransactionRepository struct {
	domain.TransactionRepository
	transactions []*domain.Transaction
	createErr    error
}

func (m *mockTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	m.transactions = append(m.transactions, tx)
	return m.createErr
}

func (m *mockTransactionRepository) GetByAccountID(ctx context.Context, accountID string) ([]*domain.Transaction, error) {
	return m.transactions, nil
}

func TestPaymentUsecase_ProcessPayment(t *testing.T) {
	fromAcc := "11111111-1111-1111-1111-111111111111"
	toAcc := "22222222-2222-2222-2222-222222222222"

	t.Run("successful payment - queries and returns success transaction", func(t *testing.T) {
		accRepo := &mockAccountRepository{}
		txRepo := &mockTransactionRepository{}
		
		// Simulate GORM auto-logging success transaction
		successTx := &domain.Transaction{
			ID:              "tx-uuid-123",
			FromAccountID:   fromAcc,
			ToAccountID:     toAcc,
			Amount:          5000,
			TransactionType: "transfer",
			Status:          domain.TransactionStateSuccess,
			Description:     "Rent",
			CreatedAt:       time.Now(),
		}
		_ = txRepo.Create(context.Background(), successTx)

		u := NewPaymentUsecase(accRepo, txRepo)
		tx, err := u.ProcessPayment(context.Background(), fromAcc, toAcc, 5000, "Rent")

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if tx.Status != domain.TransactionStateSuccess {
			t.Errorf("expected transaction status %q, got %q", domain.TransactionStateSuccess, tx.Status)
		}
		if tx.ID != "tx-uuid-123" {
			t.Errorf("expected queried transaction ID 'tx-uuid-123', got %q", tx.ID)
		}
	})

	t.Run("failed payment - writes failed record to DB for auditing", func(t *testing.T) {
		accRepo := &mockAccountRepository{
			transferErr: errors.New("insufficient balance"),
		}
		txRepo := &mockTransactionRepository{}

		u := NewPaymentUsecase(accRepo, txRepo)
		tx, err := u.ProcessPayment(context.Background(), fromAcc, toAcc, 5000, "Rent")

		if err == nil {
			t.Error("expected error, got nil")
		}
		if err.Error() != "insufficient balance" {
			t.Errorf("expected 'insufficient balance' error, got %v", err)
		}

		if tx.Status != domain.TransactionStateFailed {
			t.Errorf("expected status %q, got %q", domain.TransactionStateFailed, tx.Status)
		}
		if !strings.Contains(tx.Description, "Failed: insufficient balance") {
			t.Errorf("expected fail reason in description, got %q", tx.Description)
		}

		// Ensure the failed transaction record WAS saved to DB
		if len(txRepo.transactions) != 1 {
			t.Errorf("expected 1 record saved to DB, got %d", len(txRepo.transactions))
		}
		savedTx := txRepo.transactions[0]
		if savedTx.Status != domain.TransactionStateFailed {
			t.Errorf("expected saved status to be %q, got %q", domain.TransactionStateFailed, savedTx.Status)
		}
	})
}
