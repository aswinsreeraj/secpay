package domain

import (
	"context"
	"sync/atomic"
	"time"
)

// PaymentJob represents a task payload queued for asynchronous processing.
type PaymentJob struct {
	ID             string        `json:"id"`
	FromAccountID  string        `json:"from_account_id"`
	ToAccountID    string        `json:"to_account_id"`
	Amount         int64         `json:"amount"`
	Description    string        `json:"description"`
	IdempotencyKey string        `json:"idempotency_key"`
	RetryCount     int           `json:"retry_count"`
	MaxRetries     int           `json:"max_retries"`
	Backoff        time.Duration `json:"backoff"`
	Ctx            context.Context
}

// WorkerMetrics tracks concurrent work operations utilizing atomic operations to guarantee absolute thread safety.
type WorkerMetrics struct {
	ActiveWorkers int64 `json:"active_workers"`
	JobsProcessed int64 `json:"jobs_processed"`
	JobsFailed    int64 `json:"jobs_failed"`
	JobsRetried   int64 `json:"jobs_retried"`
}

// IncrementActive atomically increments active worker count.
func (m *WorkerMetrics) IncrementActive() {
	atomic.AddInt64(&m.ActiveWorkers, 1)
}

// DecrementActive atomically decrements active worker count.
func (m *WorkerMetrics) DecrementActive() {
	atomic.AddInt64(&m.ActiveWorkers, -1)
}

// IncrementProcessed atomically increments processed jobs.
func (m *WorkerMetrics) IncrementProcessed() {
	atomic.AddInt64(&m.JobsProcessed, 1)
}

// IncrementFailed atomically increments failed jobs.
func (m *WorkerMetrics) IncrementFailed() {
	atomic.AddInt64(&m.JobsFailed, 1)
}

// IncrementRetried atomically increments retried jobs.
func (m *WorkerMetrics) IncrementRetried() {
	atomic.AddInt64(&m.JobsRetried, 1)
}
