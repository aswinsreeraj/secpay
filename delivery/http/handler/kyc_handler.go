package handler

import (
	"net/http"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// KYCHandler holds the Gin handlers for updating and verifying KYC compliance.
type KYCHandler struct {
	userUsecase usecase.UserUsecase
}

// NewKYCHandler initializes KYCHandler.
func NewKYCHandler(userUsecase usecase.UserUsecase) *KYCHandler {
	return &KYCHandler{userUsecase: userUsecase}
}

type KYCUpdateRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateKYC mocks updates to a user's KYC status.
func (h *KYCHandler) UpdateKYC(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
		return
	}

	var req KYCUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.userUsecase.UpdateKYC(c.Request.Context(), userID, req.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "KYC status updated successfully",
		"kyc_status": req.Status,
	})
}
