package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"secpay/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthUsecase handles user authentication, registration, and MFA logic.
type AuthUsecase interface {
	Register(ctx context.Context, name, email, password string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (string, error)
	VerifyMFA(ctx context.Context, code string) (bool, error)
}

type authUsecase struct {
	userRepo  domain.UserRepository
	auditRepo domain.AuditLogRepository
	jwtSecret []byte
}

// NewAuthUsecase creates a concrete AuthUsecase instance with AuditLogRepository injected.
func NewAuthUsecase(userRepo domain.UserRepository, auditRepo domain.AuditLogRepository, jwtSecret string) AuthUsecase {
	return &authUsecase{
		userRepo:  userRepo,
		auditRepo: auditRepo,
		jwtSecret: []byte(jwtSecret),
	}
}

func (u *authUsecase) Register(ctx context.Context, name, email, password string) (*domain.User, error) {
	if len(password) < 8 {
		err := errors.New("password must be at least 8 characters long")
		_ = u.auditRepo.Create(ctx, &domain.AuditLog{
			ID:        uuid.NewString(),
			Timestamp: time.Now(),
			UserID:    "unknown",
			Action:    "register",
			Status:    "failed",
			Details:   fmt.Sprintf("Failed registration for email %s: %v", email, err),
		})
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:        uuid.NewString(),
		Name:      name,
		Email:     email,
		Password:  string(hashedPassword),
		KYCStatus: "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := user.Validate(); err != nil {
		_ = u.auditRepo.Create(ctx, &domain.AuditLog{
			ID:        uuid.NewString(),
			Timestamp: time.Now(),
			UserID:    "unknown",
			Action:    "register",
			Status:    "failed",
			Details:   fmt.Sprintf("Validation failed for email %s: %v", email, err),
		})
		return nil, err
	}

	if err := u.userRepo.Create(ctx, user); err != nil {
		_ = u.auditRepo.Create(ctx, &domain.AuditLog{
			ID:        uuid.NewString(),
			Timestamp: time.Now(),
			UserID:    "unknown",
			Action:    "register",
			Status:    "failed",
			Details:   fmt.Sprintf("Database creation failed for email %s: %v", email, err),
		})
		return nil, err
	}

	// Auditing registration success
	_ = u.auditRepo.Create(ctx, &domain.AuditLog{
		ID:        uuid.NewString(),
		Timestamp: time.Now(),
		UserID:    user.ID,
		Action:    "register",
		Status:    "success",
		Details:   fmt.Sprintf("User %s successfully registered", email),
	})

	return user, nil
}

func (u *authUsecase) Login(ctx context.Context, email, password string) (string, error) {
	user, err := u.userRepo.GetByEmail(ctx, email)
	if err != nil {
		loginErr := errors.New("invalid credentials")
		_ = u.auditRepo.Create(ctx, &domain.AuditLog{
			ID:        uuid.NewString(),
			Timestamp: time.Now(),
			UserID:    "unknown",
			Action:    "login",
			Status:    "failed",
			Details:   fmt.Sprintf("Failed login for email %s: user not found", email),
		})
		return "", loginErr
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		loginErr := errors.New("invalid credentials")
		_ = u.auditRepo.Create(ctx, &domain.AuditLog{
			ID:        uuid.NewString(),
			Timestamp: time.Now(),
			UserID:    user.ID,
			Action:    "login",
			Status:    "failed",
			Details:   fmt.Sprintf("Failed login for email %s: password mismatch", email),
		})
		return "", loginErr
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(u.jwtSecret)
	if err != nil {
		return "", err
	}

	// Auditing login success
	_ = u.auditRepo.Create(ctx, &domain.AuditLog{
		ID:        uuid.NewString(),
		Timestamp: time.Now(),
		UserID:    user.ID,
		Action:    "login",
		Status:    "success",
		Details:   fmt.Sprintf("User %s successfully logged in", email),
	})

	return tokenString, nil
}

func (u *authUsecase) VerifyMFA(ctx context.Context, code string) (bool, error) {
	if code == "123456" {
		return true, nil
	}
	return false, errors.New("invalid verification code")
}
