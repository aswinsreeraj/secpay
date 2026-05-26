package postgres

import (
	"context"
	"secpay/domain"

	"gorm.io/gorm"
)

type postgresAuditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository creates a concrete instance of AuditLogRepository using GORM.
func NewAuditLogRepository(db *gorm.DB) domain.AuditLogRepository {
	return &postgresAuditLogRepository{db: db}
}

func (r *postgresAuditLogRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *postgresAuditLogRepository) GetByUserID(ctx context.Context, userID string) ([]*domain.AuditLog, error) {
	var logs []*domain.AuditLog
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("timestamp desc").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
