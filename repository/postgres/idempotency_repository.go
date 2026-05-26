package postgres

import (
	"context"
	"secpay/domain"

	"gorm.io/gorm"
)

type postgresIdempotencyRepository struct {
	db *gorm.DB
}

// NewIdempotencyRepository creates a concrete instance of IdempotencyRepository using GORM.
func NewIdempotencyRepository(db *gorm.DB) domain.IdempotencyRepository {
	return &postgresIdempotencyRepository{db: db}
}

func (r *postgresIdempotencyRepository) Create(ctx context.Context, record *domain.Idempotency) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *postgresIdempotencyRepository) Get(ctx context.Context, key string) (*domain.Idempotency, error) {
	var record domain.Idempotency
	if err := r.db.WithContext(ctx).First(&record, "key = ?", key).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *postgresIdempotencyRepository) Update(ctx context.Context, record *domain.Idempotency) error {
	return r.db.WithContext(ctx).Save(record).Error
}
