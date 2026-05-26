package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "my-super-secret-key-12345"

func TestJWTAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create test token generator
	generateTestToken := func(sub, email string, exp time.Time) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":   sub,
			"email": email,
			"exp":   exp.Unix(),
		})
		tokenString, _ := token.SignedString([]byte(testSecret))
		return tokenString
	}

	t.Run("valid token injects claims and succeeds", func(t *testing.T) {
		r := gin.New()
		r.Use(JWTAuthMiddleware(testSecret))
		r.GET("/test", func(c *gin.Context) {
			userID, _ := c.Get("userID")
			email, _ := c.Get("email")
			c.JSON(http.StatusOK, gin.H{
				"userID": userID,
				"email":  email,
			})
		})

		token := generateTestToken("user-123", "test@example.com", time.Now().Add(time.Hour))
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		expectedBody := `{"email":"test@example.com","userID":"user-123"}`
		if w.Body.String() != expectedBody {
			t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
		}
	})

	t.Run("missing authorization header fails with 401", func(t *testing.T) {
		r := gin.New()
		r.Use(JWTAuthMiddleware(testSecret))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("invalid scheme fails with 401", func(t *testing.T) {
		r := gin.New()
		r.Use(JWTAuthMiddleware(testSecret))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // basic auth scheme

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("expired token fails with 401", func(t *testing.T) {
		r := gin.New()
		r.Use(JWTAuthMiddleware(testSecret))
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		token := generateTestToken("user-123", "test@example.com", time.Now().Add(-time.Hour)) // expired 1 hour ago
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})
}
