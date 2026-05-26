package middleware

import (
	"net/http"

	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

// EnsureKYCApproved intercepts requests and blocks transactions for users with incomplete or failed verification status.
func EnsureKYCApproved(userUsecase usecase.UserUsecase) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userID, ok := userIDVal.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
			c.Abort()
			return
		}

		// Retrieve user details using usecase
		user, err := userUsecase.GetByID(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "User profile not found or inaccessible"})
			c.Abort()
			return
		}

		// Block if KYC is not approved (pending or rejected)
		if user.KYCStatus != "approved" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":      "KYC verification required",
				"kyc_status": user.KYCStatus,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
