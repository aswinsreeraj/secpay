package usecase

import (
	"context"
	"errors"
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
	jwtSecret []byte
}

// NewAuthUsecase creates a concrete AuthUsecase instance.
func NewAuthUsecase(userRepo domain.UserRepository, jwtSecret string) AuthUsecase {
	return &authUsecase{
		userRepo:  userRepo,
		jwtSecret: []byte(jwtSecret),
	}
}

func (u *authUsecase) Register(ctx context.Context, name, email, password string) (*domain.User, error) {
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters long")
	}

	// Hash password securely using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:        uuid.NewString(),
		Name:      name,
		Email:     email,
		Password:  string(hashedPassword),
		KYCStatus: "pending", // Default status
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := user.Validate(); err != nil {
		return nil, err
	}

	if err := u.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (u *authUsecase) Login(ctx context.Context, email, password string) (string, error) {
	user, err := u.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	// Compare bcrypt hash with password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	// Generate standard JWT token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"exp":   time.Now().Add(time.Hour * 24).Unix(), // 24 hours expiry
	})

	tokenString, err := token.SignedString(u.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// VerifyMFA simulates Time-Based One-Time Password validation.
// For mock purposes, it accepts "123456" as the valid code.
func (u *authUsecase) VerifyMFA(ctx context.Context, code string) (bool, error) {
	if code == "123456" {
		return true, nil
	}
	return false, errors.New("invalid verification code")
}
