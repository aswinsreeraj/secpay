package handler

import (
	"context"
	"net/http"
	"time"

	"secpay/domain"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PaymentHandler handles payment routes and implements idempotency key verification.
type PaymentHandler struct {
	workerPool      usecase.WorkerPool
	idempotencyRepo domain.IdempotencyRepository
}

// NewPaymentHandler initializes PaymentHandler with the asynchronous WorkerPool.
func NewPaymentHandler(workerPool usecase.WorkerPool, idempotencyRepo domain.IdempotencyRepository) *PaymentHandler {
	return &PaymentHandler{
		workerPool:      workerPool,
		idempotencyRepo: idempotencyRepo,
	}
}

type PaymentRequest struct {
	FromAccountID string `json:"from_account_id" binding:"required,uuid"`
	ToAccountID   string `json:"to_account_id" binding:"required,uuid"`
	Amount        int64  `json:"amount" binding:"required,gt=0"` // Amount in cents/paise
	Description   string `json:"description" binding:"max=255"`
}

// ProcessPayment handles the payment sequence, checks idempotency keys, and enqueues payments asynchronously.
func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	// 1. Enforce Idempotency-Key header
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key header is required"})
		return
	}

	// 2. Validate request payload
	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// 3. Check for existing idempotency record
	record, err := h.idempotencyRepo.Get(ctx, idempotencyKey)
	if err == nil {
		// Key exists: handle duplicate request
		if record.Status == "started" {
			c.JSON(http.StatusConflict, gin.H{"error": "transaction is currently in progress"})
			return
		}

		// Replay cached completed response
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Header().Set("X-Cache-Lookup", "HIT - Idempotent Request")
		c.Writer.WriteHeader(record.ResponseCode)
		_, _ = c.Writer.Write([]byte(record.ResponseBody))
		c.Abort()
		return
	}

	// 4. Register new idempotency key with "started" status
	newRecord := &domain.Idempotency{
		Key:       idempotencyKey,
		Status:    "started",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.idempotencyRepo.Create(ctx, newRecord); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "transaction is currently in progress"})
		return
	}

	// 5. Construct and enqueue the asynchronous PaymentJob
	jobID := uuid.NewString()
	job := domain.PaymentJob{
		ID:             jobID,
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		Amount:         req.Amount,
		Description:    req.Description,
		IdempotencyKey: idempotencyKey,
		RetryCount:     0,
		MaxRetries:     3,
		Backoff:        10 * time.Millisecond, // Small initial backoff for fast concurrent testing
		Ctx:            context.Background(),  // Detach request context for async execution safety
	}

	if err := h.workerPool.Enqueue(job); err != nil {
		// Clean up idempotency lock on enqueue failure
		// Note: GORM's Update/Delete can be used here. For simplicity, set response failed
		newRecord.Status = "completed"
		newRecord.ResponseCode = http.StatusInternalServerError
		newRecord.ResponseBody = `{"error":"failed to enqueue payment job"}`
		_ = h.idempotencyRepo.Update(ctx, newRecord)

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue payment request: " + err.Error()})
		return
	}

	// 6. Return 202 Accepted immediately
	c.JSON(http.StatusAccepted, gin.H{
		"message": "Payment request accepted and is processing asynchronously",
		"job_id":  jobID,
		"status":  domain.TransactionStateInitiated,
	})
}
