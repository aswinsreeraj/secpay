package main

import (
	"log"

	"secpay/config"
	"secpay/delivery/http/handler"
	"secpay/delivery/http/middleware"
	"secpay/repository/postgres"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("Initializing SecPay platform...")

	// 1. Load application configuration
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Fatal: failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded successfully. Environment: %s", cfg.AppEnv)

	// 2. Initialize database connection & GORM schema migrations
	db, err := postgres.InitDB(cfg)
	if err != nil {
		log.Fatalf("Fatal: failed to initialize database layer: %v", err)
	}

	// 3. Initialize Clean Architecture database repositories
	userRepo := postgres.NewUserRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	txRepo := postgres.NewTransactionRepository(db)
	idempotencyRepo := postgres.NewIdempotencyRepository(db)

	// 4. Initialize Clean Architecture business usecases
	// In production, load JWT secret from secure environment configs
	jwtSecret := "secpay-super-secure-jwt-signing-secret-key-12345"
	authUsecase := usecase.NewAuthUsecase(userRepo, jwtSecret)
	userUsecase := usecase.NewUserUsecase(userRepo)
	paymentUsecase := usecase.NewPaymentUsecase(accountRepo, txRepo)

	// 5. Initialize & Start Asynchronous Worker Pool
	workerPool := usecase.NewWorkerPool(paymentUsecase, idempotencyRepo, txRepo, 4, 100)
	workerPool.Start()
	defer workerPool.Stop()

	// 6. Initialize HTTP route handlers
	authHandler := handler.NewAuthHandler(authUsecase)
	kycHandler := handler.NewKYCHandler(userUsecase)
	paymentHandler := handler.NewPaymentHandler(workerPool, idempotencyRepo)

	// 7. Setup Gin routing engine
	r := gin.Default()

	// Public Routes
	api := r.Group("/api/v1")
	{
		api.POST("/register", authHandler.Register)
		api.POST("/login", authHandler.Login)
		api.POST("/mfa/verify", authHandler.VerifyMFA)
	}

	// Protected Routes (JWT required)
	protected := api.Group("")
	protected.Use(middleware.JWTAuthMiddleware(jwtSecret))
	{
		protected.PUT("/kyc/verify", kycHandler.UpdateKYC)

		// Transaction-safe compliance routes (JWT + KYC required)
		compliant := protected.Group("")
		compliant.Use(middleware.EnsureKYCApproved(userUsecase))
		{
			compliant.POST("/payments", paymentHandler.ProcessPayment)
		}
	}

	// 8. Start HTTP server
	log.Printf("SecPay HTTP server listening on port %s...", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Fatal: failed to start SecPay server: %v", err)
	}
}
