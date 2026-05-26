package postgres

import (
	"fmt"
	"log"

	"secpay/config"
	"secpay/domain"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// InitDB initializes GORM PostgreSQL connection and runs schemas auto-migrations.
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.DBHost,
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBName,
		cfg.DBPort,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	log.Println("Running GORM auto-migrations...")
	err = db.AutoMigrate(&domain.User{}, &domain.Account{}, &domain.Transaction{}, &domain.Idempotency{}, &domain.AuditLog{})
	if err != nil {
		return nil, fmt.Errorf("failed to run database auto-migrations: %w", err)
	}

	log.Println("Database connection and migrations complete.")
	return db, nil
}
