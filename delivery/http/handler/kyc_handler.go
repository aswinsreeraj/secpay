package handler

import (
	"net/http"
	"secpay/delivery/http/response"
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
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Authentication required"})
		return
	}

	userID, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Invalid user context"})
		return
	}

	var req KYCUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	err := h.userUsecase.UpdateKYC(c.Request.Context(), userID, req.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.KYCVerifyResponse{
		Message:   "KYC status updated successfully",
		KYCStatus: req.Status,
	})
}
