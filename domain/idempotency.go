package domain

import (
	"context"
	"time"
)

// Idempotency represents a record of a processed or in-progress idempotent HTTP request.
type Idempotency struct {
	Key          string    `gorm:"primaryKey"`
	Status       string    `gorm:"type:varchar(50);not null"` // e.g. "started", "completed"
	ResponseCode int       `json:"response_code"`
	ResponseBody string    `json:"response_body"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IdempotencyRepository defines the contract for idempotency data persistence.
type IdempotencyRepository interface {
	Create(ctx context.Context, record *Idempotency) error
	Get(ctx context.Context, key string) (*Idempotency, error)
	Update(ctx context.Context, record *Idempotency) error
}
