package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"secpay/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresAccountRepository struct {
	db *gorm.DB
}

// NewAccountRepository creates a concrete instance of AccountRepository using GORM.
func NewAccountRepository(db *gorm.DB) domain.AccountRepository {
	return &postgresAccountRepository{db: db}
}

func (r *postgresAccountRepository) Create(ctx context.Context, account *domain.Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *postgresAccountRepository) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	var account domain.Account
	if err := r.db.WithContext(ctx).First(&account, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *postgresAccountRepository) GetByAccountNumber(ctx context.Context, accNum string) (*domain.Account, error) {
	var account domain.Account
	if err := r.db.WithContext(ctx).First(&account, "account_number = ?", accNum).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *postgresAccountRepository) Update(ctx context.Context, account *domain.Account) error {
	return r.db.WithContext(ctx).Save(account).Error
}

// TransferBalance performs a safe, ACID-compliant double-entry transfer between two accounts inside a database transaction block.
// It obtains pessimistic locks (FOR UPDATE) to prevent race conditions and sorts the locks lexicographically to prevent deadlocks.
func (r *postgresAccountRepository) TransferBalance(ctx context.Context, fromAccountID, toAccountID string, amount int64, description string) error {
	if fromAccountID == toAccountID {
		return errors.New("cannot transfer money to the same account")
	}
	if amount <= 0 {
		return errors.New("transfer amount must be positive")
	}

	// Use GORM's built-in transaction block. Any error returned automatically triggers a rollback.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Order account IDs lexicographically to prevent database deadlocks.
		firstLockID := fromAccountID
		secondLockID := toAccountID
		if fromAccountID > toAccountID {
			firstLockID = toAccountID
			secondLockID = fromAccountID
		}

		// 2. Fetch and lock accounts in the sorted deterministic order (Pessimistic Locking - FOR UPDATE)
		var firstAcc domain.Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&firstAcc, "id = ?", firstLockID).Error; err != nil {
			return fmt.Errorf("failed to fetch and lock account %s: %w", firstLockID, err)
		}

		var secondAcc domain.Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&secondAcc, "id = ?", secondLockID).Error; err != nil {
			return fmt.Errorf("failed to fetch and lock account %s: %w", secondLockID, err)
		}

		// 3. Assign sender and receiver roles
		var sender, receiver *domain.Account
		if fromAccountID == firstLockID {
			sender = &firstAcc
			receiver = &secondAcc
		} else {
			sender = &secondAcc
			receiver = &firstAcc
		}

		// 4. Perform business rule validations
		if sender.Balance < amount {
			return errors.New("insufficient balance")
		}

		// 5. Update balances
		sender.Balance -= amount
		receiver.Balance += amount

		// 6. Persist balance updates
		if err := tx.Save(sender).Error; err != nil {
			return fmt.Errorf("failed to deduct balance from sender: %w", err)
		}
		if err := tx.Save(receiver).Error; err != nil {
			return fmt.Errorf("failed to credit balance to receiver: %w", err)
		}

		// 7. Write audit log transaction record
		txLog := &domain.Transaction{
			ID:              uuid.NewString(),
			FromAccountID:   fromAccountID,
			ToAccountID:     toAccountID,
			Amount:          amount,
			TransactionType: "transfer",
			Status:          domain.TransactionStateSuccess,
			Description:     description,
			CreatedAt:       time.Now(),
		}
		if err := tx.Create(txLog).Error; err != nil {
			return fmt.Errorf("failed to create transaction log record: %w", err)
		}

		return nil
	})
}
