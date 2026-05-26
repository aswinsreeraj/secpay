package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"secpay/domain"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// mockAuthUsecase mock AuthUsecase for route testing
type mockAuthUsecase struct {
	usecase.AuthUsecase
}

func (m *mockAuthUsecase) Register(ctx context.Context, name, email, password string) (*domain.User, error) {
	if email == "existing@example.com" {
		return nil, errors.New("user already exists")
	}
	return &domain.User{
		ID:        "user-id-abc",
		Name:      name,
		Email:     email,
		KYCStatus: "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockAuthUsecase) Login(ctx context.Context, email, password string) (string, error) {
	if email == "correct@example.com" && password == "password123" {
		return "mock-jwt-token-string", nil
	}
	return "", errors.New("invalid credentials")
}

func (m *mockAuthUsecase) VerifyMFA(ctx context.Context, code string) (bool, error) {
	if code == "123456" {
		return true, nil
	}
	return false, errors.New("invalid verification code")
}

func TestAuthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockUsecase := &mockAuthUsecase{}
	authHandler := NewAuthHandler(mockUsecase)

	t.Run("register - success returns 201", func(t *testing.T) {
		r := gin.New()
		r.POST("/register", authHandler.Register)

		reqBody := `{"name":"Alice","email":"alice@example.com","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201 Created, got %d", w.Code)
		}
	})

	t.Run("register - failed validation returns 400", func(t *testing.T) {
		r := gin.New()
		r.POST("/register", authHandler.Register)

		reqBody := `{"name":"Alice","email":"invalid-email-format","password":"short"}`
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("login - success returns 200 and token", func(t *testing.T) {
		r := gin.New()
		r.POST("/login", authHandler.Login)

		reqBody := `{"email":"correct@example.com","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		expectedBody := `{"token":"mock-jwt-token-string"}`
		if w.Body.String() != expectedBody {
			t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
		}
	})

	t.Run("login - invalid credentials returns 401", func(t *testing.T) {
		r := gin.New()
		r.POST("/login", authHandler.Login)

		reqBody := `{"email":"wrong@example.com","password":"wrong-password"}`
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("verify MFA - success returns 200", func(t *testing.T) {
		r := gin.New()
		r.POST("/mfa/verify", authHandler.VerifyMFA)

		reqBody := `{"code":"123456"}`
		req := httptest.NewRequest(http.MethodPost, "/mfa/verify", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("verify MFA - invalid code returns 401", func(t *testing.T) {
		r := gin.New()
		r.POST("/mfa/verify", authHandler.VerifyMFA)

		reqBody := `{"code":"000000"}`
		req := httptest.NewRequest(http.MethodPost, "/mfa/verify", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})
}
