package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"secpay/delivery/http/middleware"
	"secpay/delivery/http/response"
	"secpay/domain"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// mockCanceledAccountRepository implements AccountRepository to verify it is NOT called when context is canceled.
type mockCanceledAccountRepository struct {
	domain.AccountRepository
	called bool
}

func (m *mockCanceledAccountRepository) TransferBalance(ctx context.Context, fromAccountID, toAccountID string, amount int64, description string) error {
	m.called = true
	return nil
}

// mockTransactionRepository implements TransactionRepository for tests
type mockTransactionRepository struct {
	domain.TransactionRepository
	transactions []*domain.Transaction
	createErr    error
}

func (m *mockTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	m.transactions = append(m.transactions, tx)
	return m.createErr
}

func (m *mockTransactionRepository) GetByAccountID(ctx context.Context, accountID string) ([]*domain.Transaction, error) {
	return m.transactions, nil
}

// TestPaymentIntegration_InvalidInputs exercises the input parsing and validation layer
func TestPaymentIntegration_InvalidInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pool := &mockWorkerPool{}
	repo := newMockIdempotencyRepository()
	h := NewPaymentHandler(pool, repo)

	r := gin.New()
	r.POST("/api/v1/payments", h.ProcessPayment)

	t.Run("malformed json syntax returns 400 Bad Request", func(t *testing.T) {
		malformedJSON := `{"from_account_id": "11111111-1111-1111-1111-111111111111", "to_account_id": ` // invalid json syntax
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(malformedJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "test-key-malformed-json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
		var errResp response.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to parse error response: %v", err)
		}
		if errResp.Error == "" {
			t.Errorf("expected non-empty error message, got empty")
		}
	})

	t.Run("malformed numeric amount (string instead of int) returns 400 Bad Request", func(t *testing.T) {
		payload := `{"from_account_id":"11111111-1111-1111-1111-111111111111","to_account_id":"22222222-2222-2222-2222-222222222222","amount":"five-thousand","description":"Rent"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "test-key-numeric-string")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "amount") {
			t.Errorf("expected error message to complain about amount type, got %s", w.Body.String())
		}
	})

	t.Run("invalid non-positive amount (zero) returns 400 Bad Request", func(t *testing.T) {
		payload := `{"from_account_id":"11111111-1111-1111-1111-111111111111","to_account_id":"22222222-2222-2222-2222-222222222222","amount":0,"description":"Rent"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "test-key-amount-zero")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Error:Field validation for 'Amount' failed") {
			t.Errorf("expected Amount validation error, got %s", w.Body.String())
		}
	})

	t.Run("invalid non-positive amount (negative) returns 400 Bad Request", func(t *testing.T) {
		payload := `{"from_account_id":"11111111-1111-1111-1111-111111111111","to_account_id":"22222222-2222-2222-2222-222222222222","amount":-500,"description":"Rent"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "test-key-amount-neg")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("malformed UUID for from_account_id returns 400 Bad Request", func(t *testing.T) {
		payload := `{"from_account_id":"invalid-uuid-format","to_account_id":"22222222-2222-2222-2222-222222222222","amount":1000,"description":"Rent"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "test-key-uuid-err")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "FromAccountID") {
			t.Errorf("expected error message to mention FromAccountID, got %s", w.Body.String())
		}
	})
}

// TestPaymentIntegration_AbortedContext verifies context cancellations are intercepted before database balance mutations
func TestPaymentIntegration_AbortedContext(t *testing.T) {
	fromAcc := "11111111-1111-1111-1111-111111111111"
	toAcc := "22222222-2222-2222-2222-222222222222"

	t.Run("aborted context is intercepted at usecase layer", func(t *testing.T) {
		accRepo := &mockCanceledAccountRepository{called: false}
		txRepo := &mockTransactionRepository{}

		u := usecase.NewPaymentUsecase(accRepo, txRepo)

		// Create a canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		tx, err := u.ProcessPayment(ctx, fromAcc, toAcc, 5000, "Should abort")

		if err == nil {
			t.Error("expected error due to canceled context, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if tx != nil {
			t.Errorf("expected returned transaction to be nil, got %+v", tx)
		}
		if accRepo.called {
			t.Error("expected account repository balance mutation to NOT be called, but it was!")
		}
	})
}

// TestPaymentIntegration_RecoveryMiddleware verifies unexpected panic is captured and structured 500 returned
func TestPaymentIntegration_RecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	// Register our elegant custom structured logger and recovery middleware
	r.Use(middleware.RecoveryMiddleware())

	// Endpoint that intentionally panics to test the middleware
	r.GET("/api/v1/panic-endpoint", func(c *gin.Context) {
		panic("simulated critical crash anomaly")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/panic-endpoint", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error, got %d", w.Code)
	}

	var errResp response.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error != "An unexpected internal server error occurred" {
		t.Errorf("expected elegant recovery message, got %q", errResp.Error)
	}
}

// TestPaymentIntegration_OutofOrderExecution verifies idempotency conflicts under simultaneous out-of-order requests
func TestPaymentIntegration_OutofOrderExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pool := &mockWorkerPool{}
	repo := newMockIdempotencyRepository()
	h := NewPaymentHandler(pool, repo)

	r := gin.New()
	r.POST("/api/v1/payments", h.ProcessPayment)

	fromAcc := "11111111-1111-1111-1111-111111111111"
	toAcc := "22222222-2222-2222-2222-222222222222"
	body := `{"from_account_id":"` + fromAcc + `","to_account_id":"` + toAcc + `","amount":1000,"description":"Double charge"}`

	// Perform first request
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "key-double-charge-123")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusAccepted {
		t.Errorf("first request: expected 202 StatusAccepted, got %d", w1.Code)
	}

	// Perform duplicate request immediately (simulate out-of-order/re-entrant request)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "key-double-charge-123")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate request: expected 409 StatusConflict, got %d", w2.Code)
	}

	var errResp response.ErrorResponse
	_ = json.Unmarshal(w2.Body.Bytes(), &errResp)
	if errResp.Error != "transaction is currently in progress" {
		t.Errorf("expected conflict message, got %q", errResp.Error)
	}
}
