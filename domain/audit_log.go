package domain

import (
	"context"
	"time"
)

// AuditLog represents an immutable record of system actions and compliance operations.
type AuditLog struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Timestamp time.Time `json:"timestamp" gorm:"not null;index"`
	UserID    string    `json:"user_id" gorm:"index"`
	Action    string    `json:"action" gorm:"not null;index"` // e.g., "register", "login", "payment_initiated", "payment_success", "payment_failed"
	Status    string    `json:"status" gorm:"not null"`       // e.g., "success", "failed", "retry"
	Details   string    `json:"details" gorm:"type:text"`
}

// AuditLogRepository defines the contract for persisting immutable auditing entries.
type AuditLogRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	GetByUserID(ctx context.Context, userID string) ([]*AuditLog, error)
}
