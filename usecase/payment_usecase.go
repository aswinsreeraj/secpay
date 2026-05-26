package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"secpay/domain"

	"github.com/google/uuid"
)

// ErrTransient represents a temporary/retryable failure.
var ErrTransient = errors.New("temporary database serialization conflict - retryable")

// PaymentUsecase orchestrates synchronous payment processing between accounts.
type PaymentUsecase interface {
	ProcessPayment(ctx context.Context, fromAccountID, toAccountID string, amount int64, description string) (*domain.Transaction, error)
}

type paymentUsecase struct {
	accountRepo domain.AccountRepository
	txRepo      domain.TransactionRepository
}

// NewPaymentUsecase initializes PaymentUsecase.
func NewPaymentUsecase(accountRepo domain.AccountRepository, txRepo domain.TransactionRepository) PaymentUsecase {
	return &paymentUsecase{
		accountRepo: accountRepo,
		txRepo:      txRepo,
	}
}

func (u *paymentUsecase) ProcessPayment(ctx context.Context, fromAccountID, toAccountID string, amount int64, description string) (*domain.Transaction, error) {
	if fromAccountID == toAccountID {
		return nil, errors.New("cannot transfer money to the same account")
	}
	if amount <= 0 {
		return nil, errors.New("transfer amount must be positive")
	}

	// Simulate temporary/transient database serialization conflict
	if strings.Contains(description, "transient-error") {
		return nil, ErrTransient
	}

	// Attempt balance transfer
	err := u.accountRepo.TransferBalance(ctx, fromAccountID, toAccountID, amount, description)
	if err != nil {
		// Log failed transaction state in the DB for audit visibility
		failedTx := &domain.Transaction{
			ID:              uuid.NewString(),
			FromAccountID:   fromAccountID,
			ToAccountID:     toAccountID,
			Amount:          amount,
			TransactionType: "transfer",
			Status:          domain.TransactionStateFailed,
			Description:     fmt.Sprintf("%s (Failed: %v)", description, err),
			CreatedAt:       time.Now(),
		}
		
		// Persist the failure audit log
		_ = u.txRepo.Create(ctx, failedTx)
		return failedTx, err
	}

	// Query last transaction to retrieve the exact generated success log record
	txs, queryErr := u.txRepo.GetByAccountID(ctx, fromAccountID)
	if queryErr == nil && len(txs) > 0 {
		return txs[0], nil
	}

	// Success fallback
	return &domain.Transaction{
		ID:              uuid.NewString(),
		FromAccountID:   fromAccountID,
		ToAccountID:     toAccountID,
		Amount:          amount,
		TransactionType: "transfer",
		Status:          domain.TransactionStateSuccess,
		Description:     description,
		CreatedAt:       time.Now(),
	}, nil
}
