package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"secpay/domain"
)

// mockIdempotencyRepo for async tests
type mockIdempotencyRepo struct {
	domain.IdempotencyRepository
	mu      sync.Mutex
	records map[string]*domain.Idempotency
}

func newMockIdempotencyRepo() *mockIdempotencyRepo {
	return &mockIdempotencyRepo{records: make(map[string]*domain.Idempotency)}
}

func (m *mockIdempotencyRepo) Create(ctx context.Context, record *domain.Idempotency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[record.Key] = record
	return nil
}

func (m *mockIdempotencyRepo) Get(ctx context.Context, key string) (*domain.Idempotency, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return record, nil
}

func (m *mockIdempotencyRepo) Update(ctx context.Context, record *domain.Idempotency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[record.Key] = record
	return nil
}

// mockTxRepo for async tests
type mockTxRepo struct {
	domain.TransactionRepository
	mu           sync.Mutex
	transactions []*domain.Transaction
}

func newMockTxRepo() *mockTxRepo {
	return &mockTxRepo{}
}

func (m *mockTxRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions = append(m.transactions, tx)
	return nil
}

func (m *mockTxRepo) GetByAccountID(ctx context.Context, accountID string) ([]*domain.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.transactions, nil
}

// mockAuditLogRepo for async tests
type mockAuditLogRepo struct {
	domain.AuditLogRepository
	mu   sync.Mutex
	logs []*domain.AuditLog
}

func newMockAuditLogRepo() *mockAuditLogRepo {
	return &mockAuditLogRepo{}
}

func (m *mockAuditLogRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, log)
	return nil
}

func (m *mockAuditLogRepo) GetByUserID(ctx context.Context, userID string) ([]*domain.AuditLog, error) {
	return nil, nil
}

func TestWorkerPool_ConcurrencyAndLoad(t *testing.T) {
	accRepo := &mockAccountRepository{}
	txRepo := newMockTxRepo()
	idempotencyRepo := newMockIdempotencyRepo()
	auditRepo := newMockAuditLogRepo()

	// 1. Success usecase setup
	u := NewPaymentUsecase(accRepo, txRepo)
	pool := NewWorkerPool(u, idempotencyRepo, txRepo, auditRepo, 4, 100) // 4 concurrent workers
	pool.Start()
	defer pool.Stop()

	// 2. Spawn 50 concurrent payment jobs under load to verify thread safety
	numJobs := 50
	var wg sync.WaitGroup
	wg.Add(numJobs)

	for i := 0; i < numJobs; i++ {
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("idempotency-key-%d", id)
			
			// Setup started state in DB
			_ = idempotencyRepo.Create(context.Background(), &domain.Idempotency{
				Key:    key,
				Status: "started",
			})

			job := domain.PaymentJob{
				ID:             fmt.Sprintf("job-id-%d", id),
				FromAccountID:  "acc-sender",
				ToAccountID:    "acc-receiver",
				Amount:         100,
				Description:    "Concurrent load payment",
				IdempotencyKey: key,
				RetryCount:     0,
				MaxRetries:     2,
				Backoff:        2 * time.Millisecond,
			}
			
			_ = pool.Enqueue(job)
		}(i)
	}

	wg.Wait()

	// Allow some time for background workers to empty the queue
	time.Sleep(100 * time.Millisecond)

	metrics := pool.GetMetrics()
	if metrics.JobsProcessed != int64(numJobs) {
		t.Errorf("expected %d jobs processed, got %d", numJobs, metrics.JobsProcessed)
	}

	// Verify all idempotency statuses are "completed"
	idempotencyRepo.mu.Lock()
	for _, rec := range idempotencyRepo.records {
		if rec.Status != "completed" {
			t.Errorf("expected idempotency status 'completed' for key %s, got %s", rec.Key, rec.Status)
		}
	}
	idempotencyRepo.mu.Unlock()
}

func TestWorkerPool_ExponentialBackoffRetry(t *testing.T) {
	accRepo := &mockAccountRepository{}
	txRepo := newMockTxRepo()
	idempotencyRepo := newMockIdempotencyRepo()
	auditRepo := newMockAuditLogRepo()

	u := NewPaymentUsecase(accRepo, txRepo)
	pool := NewWorkerPool(u, idempotencyRepo, txRepo, auditRepo, 2, 10)
	pool.Start()
	defer pool.Stop()

	// Setup started state in DB
	key := "key-transient-123"
	_ = idempotencyRepo.Create(context.Background(), &domain.Idempotency{
		Key:    key,
		Status: "started",
	})

	// Submit job with "transient-error" inside description to trigger transient failure retries
	job := domain.PaymentJob{
		ID:             "job-transient",
		FromAccountID:  "acc-sender",
		ToAccountID:    "acc-receiver",
		Amount:         100,
		Description:    "Simulated transient-error test",
		IdempotencyKey: key,
		RetryCount:     0,
		MaxRetries:     2,                   // Will try original + 2 retries = 3 attempts total
		Backoff:        5 * time.Millisecond, // Speed up testing backoff sleep
	}

	_ = pool.Enqueue(job)

	// Allow enough time for all retries and exponential backoff timers to trigger
	time.Sleep(100 * time.Millisecond)

	metrics := pool.GetMetrics()
	if metrics.JobsRetried != 2 {
		t.Errorf("expected 2 job retries, got %d", metrics.JobsRetried)
	}
	if metrics.JobsFailed != 1 {
		t.Errorf("expected 1 final job failure, got %d", metrics.JobsFailed)
	}

	// Ensure the failed transaction audit record was correctly persisted
	txRepo.mu.Lock()
	if len(txRepo.transactions) != 1 {
		t.Errorf("expected failed transaction saved for audit, got %d", len(txRepo.transactions))
	} else {
		savedTx := txRepo.transactions[0]
		if savedTx.Status != domain.TransactionStateFailed {
			t.Errorf("expected saved status to be 'failed', got %s", savedTx.Status)
		}
	}
	txRepo.mu.Unlock()

	// Ensure cached idempotency response code is 400 Bad Request
	rec, _ := idempotencyRepo.Get(context.Background(), key)
	if rec.Status != "completed" || rec.ResponseCode != 400 {
		t.Errorf("expected status 'completed' and code 400, got status %q code %d", rec.Status, rec.ResponseCode)
	}

	// Ensure audit entries exist (attempt, retries, and failure log)
	auditRepo.mu.Lock()
	if len(auditRepo.logs) < 4 { // 1 attempt + 2 retries + 1 terminal fail = 4 logs minimum
		t.Errorf("expected at least 4 audit logs written, got %d", len(auditRepo.logs))
	}
	auditRepo.mu.Unlock()
}
