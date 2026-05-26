package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"secpay/domain"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// mockUserUsecase implements usecase.UserUsecase for testing purposes
type mockUserUsecase struct {
	usecase.UserUsecase
}

func (m *mockUserUsecase) GetByID(ctx context.Context, id string) (*domain.User, error) {
	switch id {
	case "approved-user":
		return &domain.User{ID: id, Name: "Approved User", Email: "approved@example.com", KYCStatus: "approved"}, nil
	case "pending-user":
		return &domain.User{ID: id, Name: "Pending User", Email: "pending@example.com", KYCStatus: "pending"}, nil
	case "rejected-user":
		return &domain.User{ID: id, Name: "Rejected User", Email: "rejected@example.com", KYCStatus: "rejected"}, nil
	default:
		return nil, errors.New("user not found")
	}
}

func (m *mockUserUsecase) UpdateKYC(ctx context.Context, id string, status string) error {
	return nil
}

func TestEnsureKYCApproved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockUsecase := &mockUserUsecase{}

	setupRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			// Mock JWT claims injection
			userID := c.GetHeader("X-Test-User-ID")
			if userID != "" {
				c.Set("userID", userID)
			}
			c.Next()
		})
		r.Use(EnsureKYCApproved(mockUsecase))
		r.GET("/protected-transfer", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "success"})
		})
		return r
	}

	t.Run("approved user is allowed to proceed", func(t *testing.T) {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, "/protected-transfer", nil)
		req.Header.Set("X-Test-User-ID", "approved-user")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("pending user is blocked with 403", func(t *testing.T) {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, "/protected-transfer", nil)
		req.Header.Set("X-Test-User-ID", "pending-user")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})

	t.Run("rejected user is blocked with 403", func(t *testing.T) {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, "/protected-transfer", nil)
		req.Header.Set("X-Test-User-ID", "rejected-user")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})

	t.Run("unauthenticated request is blocked", func(t *testing.T) {
		r := setupRouter()
		req := httptest.NewRequest(http.MethodGet, "/protected-transfer", nil) // No X-Test-User-ID header

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})
}
