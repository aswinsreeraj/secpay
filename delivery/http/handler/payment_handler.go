package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"secpay/domain"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// PaymentHandler handles payment routes and implements idempotency key verification.
type PaymentHandler struct {
	paymentUsecase  usecase.PaymentUsecase
	idempotencyRepo domain.IdempotencyRepository
}

// NewPaymentHandler initializes PaymentHandler.
func NewPaymentHandler(paymentUsecase usecase.PaymentUsecase, idempotencyRepo domain.IdempotencyRepository) *PaymentHandler {
	return &PaymentHandler{
		paymentUsecase:  paymentUsecase,
		idempotencyRepo: idempotencyRepo,
	}
}

type PaymentRequest struct {
	FromAccountID string `json:"from_account_id" binding:"required,uuid"`
	ToAccountID   string `json:"to_account_id" binding:"required,uuid"`
	Amount        int64  `json:"amount" binding:"required,gt=0"` // Amount in cents/paise
	Description   string `json:"description" binding:"max=255"`
}

// ProcessPayment handles the payment sequence, checks idempotency keys, and persists outcomes.
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

	// Helper function to update idempotency cache and return response
	sendResponse := func(statusCode int, responseObj interface{}) {
		responseBytes, _ := json.Marshal(responseObj)
		
		newRecord.Status = "completed"
		newRecord.ResponseCode = statusCode
		newRecord.ResponseBody = string(responseBytes)
		newRecord.UpdatedAt = time.Now()
		
		_ = h.idempotencyRepo.Update(ctx, newRecord)

		c.JSON(statusCode, responseObj)
	}

	// 5. Execute payment usecase
	txRecord, err := h.paymentUsecase.ProcessPayment(ctx, req.FromAccountID, req.ToAccountID, req.Amount, req.Description)
	if err != nil {
		// Return 400 Bad Request with the failed transaction details for audit trails
		sendResponse(http.StatusBadRequest, gin.H{
			"error":       err.Error(),
			"transaction": txRecord,
		})
		return
	}

	// Return 200 OK with success transaction record details
	sendResponse(http.StatusOK, gin.H{
		"message":     "Payment processed successfully",
		"transaction": txRecord,
	})
}
