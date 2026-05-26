package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func TestRateLimiterMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("burst within limit succeeds, exceeds limit returns 429", func(t *testing.T) {
		r := gin.New()
		
		// 5 requests per second, burst size of 5 tokens
		r.Use(RateLimiterMiddleware(rate.Limit(5), 5))
		
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// 1. Send 5 rapid requests - all should succeed (status 200)
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", "192.168.1.100") // Simulate static client IP
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("expected 200 OK on request %d, got %d", i+1, w.Code)
			}
		}

		// 2. Send 6th request immediately - should be blocked (status 429)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.100")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("expected 429 Too Many Requests, got %d", w.Code)
		}
	})

	t.Run("distinct IPs have independent buckets", func(t *testing.T) {
		r := gin.New()
		r.Use(RateLimiterMiddleware(rate.Limit(1), 1)) // 1 rps, 1 burst
		r.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// Request from IP A - succeeds
		reqA1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		reqA1.Header.Set("X-Forwarded-For", "10.0.0.1")
		wA1 := httptest.NewRecorder()
		r.ServeHTTP(wA1, reqA1)
		if wA1.Code != http.StatusOK {
			t.Errorf("expected IP A 1st request 200, got %d", wA1.Code)
		}

		// Request from IP B - succeeds (doesn't share token bucket with A!)
		reqB1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		reqB1.Header.Set("X-Forwarded-For", "10.0.0.2")
		wB1 := httptest.NewRecorder()
		r.ServeHTTP(wB1, reqB1)
		if wB1.Code != http.StatusOK {
			t.Errorf("expected IP B 1st request 200, got %d", wB1.Code)
		}

		// 2nd request from IP A immediately - fails (depleted A's bucket)
		reqA2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		reqA2.Header.Set("X-Forwarded-For", "10.0.0.1")
		wA2 := httptest.NewRecorder()
		r.ServeHTTP(wA2, reqA2)
		if wA2.Code != http.StatusTooManyRequests {
			t.Errorf("expected IP A 2nd request 429, got %d", wA2.Code)
		}
	})
}
