package handler

import (
	"net/http"

	"secpay/delivery/http/response"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// AuthHandler holds the Gin handlers for registration, login, and MFA verification.
type AuthHandler struct {
	authUsecase usecase.AuthUsecase
}

// NewAuthHandler initializes AuthHandler.
func NewAuthHandler(authUsecase usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{authUsecase: authUsecase}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Register registers a new user.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	user, err := h.authUsecase.Register(c.Request.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response.RegisterResponse{
		Message: "User registered successfully",
		User: response.UserSummary{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			KYCStatus: user.KYCStatus,
		},
	})
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates a user and returns a JWT token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	token, err := h.authUsecase.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.LoginResponse{
		Token: token,
	})
}

type MFARequest struct {
	Code string `json:"code" binding:"required"`
}

// VerifyMFA simulates multi-factor authentication (MFA) validation.
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	var req MFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	valid, err := h.authUsecase.VerifyMFA(c.Request.Context(), req.Code)
	if err != nil || !valid {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Invalid or expired MFA code"})
		return
	}

	c.JSON(http.StatusOK, response.MFAVerifyResponse{
		Message: "MFA verified successfully",
		Status:  "authorized",
	})
}
