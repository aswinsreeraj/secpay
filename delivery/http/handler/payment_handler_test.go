package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"secpay/domain"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// mockPaymentUsecase mock PaymentUsecase for API testing
type mockPaymentUsecase struct {
	usecase.PaymentUsecase
	processErr error
	transactionsCalled int
}

func (m *mockPaymentUsecase) ProcessPayment(ctx context.Context, fromAccountID, toAccountID string, amount int64, description string) (*domain.Transaction, error) {
	m.transactionsCalled++
	if m.processErr != nil {
		return &domain.Transaction{
			ID:              "tx-fail-123",
			FromAccountID:   fromAccountID,
			ToAccountID:     toAccountID,
			Amount:          amount,
			TransactionType: "transfer",
			Status:          domain.TransactionStateFailed,
			Description:     description + " (Failed)",
			CreatedAt:       time.Now(),
		}, m.processErr
	}
	return &domain.Transaction{
		ID:              "tx-success-123",
		FromAccountID:   fromAccountID,
		ToAccountID:     toAccountID,
		Amount:          amount,
		TransactionType: "transfer",
		Status:          domain.TransactionStateSuccess,
		Description:     description,
		CreatedAt:       time.Now(),
	}, nil
}

// mockIdempotencyRepository mock IdempotencyRepository for API testing
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
		u := &mockPaymentUsecase{}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(u, repo)

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

	t.Run("successful processing - saves completed response", func(t *testing.T) {
		u := &mockPaymentUsecase{}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(u, repo)

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
		if !strings.Contains(w.Body.String(), "Payment processed successfully") {
			t.Errorf("expected success response, got %s", w.Body.String())
		}

		// Verify idempotency record state
		record, err := repo.Get(context.Background(), "key-123")
		if err != nil {
			t.Fatalf("expected idempotency record to exist, got %v", err)
		}
		if record.Status != "completed" {
			t.Errorf("expected status 'completed', got %q", record.Status)
		}
		if record.ResponseCode != http.StatusOK {
			t.Errorf("expected response code 200, got %d", record.ResponseCode)
		}
		if !strings.Contains(record.ResponseBody, "Payment processed successfully") {
			t.Errorf("expected cached response body to contain success message, got %q", record.ResponseBody)
		}
	})

	t.Run("duplicate request - replays cached completed response", func(t *testing.T) {
		u := &mockPaymentUsecase{}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(u, repo)

		r := gin.New()
		r.POST("/api/v1/payments", h.ProcessPayment)

		// 1. Submit first request
		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(validBody))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Idempotency-Key", "key-123")

		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		// 2. Submit duplicate request with SAME key
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(validBody))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Idempotency-Key", "key-123")

		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Errorf("expected replayed 200 OK, got %d", w2.Code)
		}
		if w2.Header().Get("X-Cache-Lookup") != "HIT - Idempotent Request" {
			t.Errorf("expected cache lookup HIT header, got %q", w2.Header().Get("X-Cache-Lookup"))
		}

		// Ensure usecase was only called ONCE
		if u.transactionsCalled != 1 {
			t.Errorf("expected usecase to be called exactly 1 time, got %d", u.transactionsCalled)
		}
	})

	t.Run("concurrent processing - returns 409 Conflict", func(t *testing.T) {
		u := &mockPaymentUsecase{}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(u, repo)

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

	t.Run("payment failure - returns 400 Bad Request and caches fail response", func(t *testing.T) {
		u := &mockPaymentUsecase{
			processErr: errors.New("insufficient balance"),
		}
		repo := newMockIdempotencyRepository()
		h := NewPaymentHandler(u, repo)

		r := gin.New()
		r.POST("/api/v1/payments", h.ProcessPayment)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(validBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "key-failed")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "insufficient balance") {
			t.Errorf("expected balance error in body, got %q", w.Body.String())
		}

		// Verify cached response state is completed with code 400
		record, _ := repo.Get(context.Background(), "key-failed")
		if record.Status != "completed" {
			t.Errorf("expected status 'completed', got %q", record.Status)
		}
		if record.ResponseCode != http.StatusBadRequest {
			t.Errorf("expected cached code 400, got %d", record.ResponseCode)
		}
	})
}
