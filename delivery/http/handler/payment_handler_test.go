package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"secpay/domain"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// mockWorkerPool mock WorkerPool for handler testing
type mockWorkerPool struct {
	usecase.WorkerPool
	jobs       []domain.PaymentJob
	enqueueErr error
}

func (m *mockWorkerPool) Enqueue(job domain.PaymentJob) error {
	if m.enqueueErr != nil {
		return m.enqueueErr
	}
	m.jobs = append(m.jobs, job)
	return nil
}

// mockIdempotencyRepository mock IdempotencyRepository for handler testing
type mockIdempotencyRepository struct {
	domain.IdempotencyRepository
	records map[string]*domain.Idempotency
}

func newMockIdempotencyRepository() *mockIdempotencyRepository {
	return &mockIdempotencyRepository{records: make(map[string]*domain.Idempotency)}
}

func (m *mockIdempotencyRepository) Create(ctx context.Context, record *domain.Idempotency) error {
	if _, ok := m.records[record.Key]; ok {
		return errors.New("unique constraint violation - duplicate key")
	}
	m.records[record.Key] = record
	return nil
}

func (m *mockIdempotencyRepository) Get(ctx context.Context, key string) (*domain.Idempotency, error) {
	record, ok := m.records[key]
	if !ok {
		return nil, errors.New("record not found")
	}
	return record, nil
}

func (m *mockIdempotencyRepository) Update(ctx context.Context, record *domain.Idempotency) error {
	m.records[record.Key] = record
	return nil
}

func TestPaymentHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fromAcc := "11111111-1111-1111-1111-111111111111"
	toAcc := "22222222-2222-2222-2222-222222222222"
	validBody := `{"from_account_id":"` + fromAcc + `","to_account_id":"` + toAcc + `","amount":5000,"description":"Rent"}`

	t.Run("missing idempotency-key header fails with 400", func(t *testing.T) {
		pool := &mockWorkerPool{}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(pool, repo)

		r := gin.New()
		r.POST("/api/v1/payments", h.ProcessPayment)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(validBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Idempotency-Key header is required") {
			t.Errorf("expected header required error message, got %q", w.Body.String())
		}
	})

	t.Run("successful enqueue - returns 202 Accepted and creates started record", func(t *testing.T) {
		pool := &mockWorkerPool{}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(pool, repo)

		r := gin.New()
		r.POST("/api/v1/payments", h.ProcessPayment)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(validBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "key-123")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("expected 202 Accepted, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Payment request accepted and is processing asynchronously") {
			t.Errorf("expected accepted message, got %s", w.Body.String())
		}

		// Verify idempotency record state
		record, err := repo.Get(context.Background(), "key-123")
		if err != nil {
			t.Fatalf("expected idempotency record to exist, got %v", err)
		}
		if record.Status != "started" {
			t.Errorf("expected status 'started', got %q", record.Status)
		}

		// Verify job is enqueued
		if len(pool.jobs) != 1 {
			t.Errorf("expected 1 job enqueued, got %d", len(pool.jobs))
		}
		enqueuedJob := pool.jobs[0]
		if enqueuedJob.Amount != 5000 || enqueuedJob.Description != "Rent" {
			t.Errorf("enqueued job fields mismatch, got %+v", enqueuedJob)
		}
	})

	t.Run("concurrent processing - returns 409 Conflict", func(t *testing.T) {
		pool := &mockWorkerPool{}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(pool, repo)

		// Add pre-existing "started" record
		_ = repo.Create(context.Background(), &domain.Idempotency{
			Key:    "key-123",
			Status: "started",
		})

		r := gin.New()
		r.POST("/api/v1/payments", h.ProcessPayment)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(validBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "key-123")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "transaction is currently in progress") {
			t.Errorf("expected in progress error, got %s", w.Body.String())
		}
	})

	t.Run("replay cached response - returns replayed completed response", func(t *testing.T) {
		pool := &mockWorkerPool{}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(pool, repo)

		// Add pre-existing completed record
		_ = repo.Create(context.Background(), &domain.Idempotency{
			Key:          "key-123",
			Status:       "completed",
			ResponseCode: http.StatusOK,
			ResponseBody: `{"message":"Replayed successfully"}`,
		})

		r := gin.New()
		r.POST("/api/v1/payments", h.ProcessPayment)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(validBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "key-123")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		if w.Header().Get("X-Cache-Lookup") != "HIT - Idempotent Request" {
			t.Errorf("expected cache lookup HIT header, got %q", w.Header().Get("X-Cache-Lookup"))
		}
		if w.Body.String() != `{"message":"Replayed successfully"}` {
			t.Errorf("expected replayed body, got %q", w.Body.String())
		}

		// Verify no new job was enqueued
		if len(pool.jobs) != 0 {
			t.Errorf("expected 0 jobs enqueued, got %d", len(pool.jobs))
		}
	})
}
