package domain

import (
	"testing"
	"time"
)

func TestUser_Validate(t *testing.T) {
	validUser := &User{
		ID:        "b8c66e92-d6c5-4ad8-a89c-a1112b36bbf4",
		Name:      "John Doe",
		Email:     "john.doe@example.com",
		Password:  "password123",
		KYCStatus: "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	t.Run("valid user", func(t *testing.T) {
		if err := validUser.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		invalidUser := *validUser
		invalidUser.Email = "invalid-email"
		if err := invalidUser.Validate(); err == nil {
			t.Error("expected error for invalid email, got nil")
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		invalidUser := *validUser
		invalidUser.ID = "invalid-uuid"
		if err := invalidUser.Validate(); err == nil {
			t.Error("expected error for invalid UUID, got nil")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		invalidUser := *validUser
		invalidUser.Name = ""
		if err := invalidUser.Validate(); err == nil {
			t.Error("expected error for missing name, got nil")
		}
	})

	t.Run("name too short", func(t *testing.T) {
		invalidUser := *validUser
		invalidUser.Name = "A"
		if err := invalidUser.Validate(); err == nil {
			t.Error("expected error for name too short, got nil")
		}
	})

	t.Run("password too short", func(t *testing.T) {
		invalidUser := *validUser
		invalidUser.Password = "1234567"
		if err := invalidUser.Validate(); err == nil {
			t.Error("expected error for password too short, got nil")
		}
	})

	t.Run("invalid KYC status", func(t *testing.T) {
		invalidUser := *validUser
		invalidUser.KYCStatus = "unknown"
		if err := invalidUser.Validate(); err == nil {
			t.Error("expected error for invalid KYC status, got nil")
		}
	})
}
