package usecase

import (
	"context"
	"errors"
	"time"

	"secpay/domain"
)

// UserUsecase handles basic user updates and KYC status adjustments.
type UserUsecase interface {
	GetByID(ctx context.Context, id string) (*domain.User, error)
	UpdateKYC(ctx context.Context, id string, status string) error
}

type userUsecase struct {
	userRepo domain.UserRepository
}

// NewUserUsecase creates a concrete UserUsecase instance.
func NewUserUsecase(userRepo domain.UserRepository) UserUsecase {
	return &userUsecase{
		userRepo: userRepo,
	}
}

func (u *userUsecase) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return u.userRepo.GetByID(ctx, id)
}

func (u *userUsecase) UpdateKYC(ctx context.Context, id string, status string) error {
	if status != "pending" && status != "approved" && status != "rejected" {
		return errors.New("invalid KYC status value")
	}

	user, err := u.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	user.KYCStatus = status
	user.UpdatedAt = time.Now()
	return u.userRepo.Update(ctx, user)
}
