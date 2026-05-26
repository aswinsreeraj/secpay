package postgres

import (
	"context"
	"secpay/domain"

	"gorm.io/gorm"
)

type postgresTransactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a concrete instance of TransactionRepository using GORM.
func NewTransactionRepository(db *gorm.DB) domain.TransactionRepository {
	return &postgresTransactionRepository{db: db}
}

func (r *postgresTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *postgresTransactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	var dbTx domain.Transaction
	if err := r.db.WithContext(ctx).First(&dbTx, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &dbTx, nil
}

func (r *postgresTransactionRepository) GetByAccountID(ctx context.Context, accountID string) ([]*domain.Transaction, error) {
	var txs []*domain.Transaction
	if err := r.db.WithContext(ctx).Where("from_account_id = ? OR to_account_id = ?", accountID, accountID).Order("created_at desc").Find(&txs).Error; err != nil {
		return nil, err
	}
	return txs, nil
}
