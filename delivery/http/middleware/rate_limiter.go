package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter maintains thread-safe maps linking client IPs to individual token-bucket limiters.
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  sync.RWMutex
	r   rate.Limit
	b   int
}

// NewIPRateLimiter initializes the IPRateLimiter struct.
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		r:   r,
		b:   b,
	}
}

// GetLimiter retrieves or creates the token-bucket rate limiter for a specific client IP.
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.ips[ip]
	i.mu.RUnlock()

	if exists {
		return limiter
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	// Double-check lock check to prevent race conditions during concurrent creates
	limiter, exists = i.ips[ip]
	if exists {
		return limiter
	}

	limiter = rate.NewLimiter(i.r, i.b)
	i.ips[ip] = limiter
	return limiter
}

// RateLimiterMiddleware returns a Gin middleware enforcing IP-based token-bucket rate limits globally.
func RateLimiterMiddleware(r rate.Limit, b int) gin.HandlerFunc {
	limiter := NewIPRateLimiter(r, b)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		lim := limiter.GetLimiter(ip)
		if !lim.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please slow down and try again.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
