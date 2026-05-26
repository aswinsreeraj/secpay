package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	
	"secpay/delivery/http/response"

	"github.com/gin-gonic/gin"
)

// RecoveryMiddleware gracefully intercepts panics in HTTP requests, logs stack traces structuredly, and returns a 500 error.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stackTrace := string(debug.Stack())
				
				slog.Error("CRITICAL: Panic recovered in HTTP request handler",
					slog.Any("error", err),
					slog.String("stack_trace", stackTrace),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, response.ErrorResponse{
					Error: "An unexpected internal server error occurred",
				})
			}
		}()
		c.Next()
	}
}
