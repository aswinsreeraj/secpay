package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// StructuredLogger returns a Gin middleware that logs incoming HTTP requests using log/slog.
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		errs := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// Construct structured log fields
		fields := []interface{}{
			slog.Int("status", status),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
			slog.String("user_agent", userAgent),
		}

		if query != "" {
			fields = append(fields, slog.String("query", query))
		}

		if errs != "" {
			fields = append(fields, slog.String("error", errs))
			slog.Error("HTTP Request Failure", fields...)
			return
		}

		// Log severity based on HTTP status
		if status >= 500 {
			slog.Error("HTTP Server Error", fields...)
		} else if status >= 400 {
			slog.Warn("HTTP Client Error", fields...)
		} else {
			slog.Info("HTTP Request Success", fields...)
		}
	}
}
