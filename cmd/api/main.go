package main

import (
	"log"
	"log/slog"
	"os"

	"secpay/config"
	"secpay/delivery/http/handler"
	"secpay/delivery/http/middleware"
	"secpay/repository/postgres"
	"secpay/usecase"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Initialize structured JSON logger globally via slog
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Initializing SecPay platform...")

	// 2. Load application configuration
	cfg, err := config.LoadConfig(".")
	if err != nil {
		slog.Error("Fatal: failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("Configuration loaded successfully", slog.String("env", cfg.AppEnv))

	// 3. Initialize database connection & GORM schema migrations
	db, err := postgres.InitDB(cfg)
	if err != nil {
		slog.Error("Fatal: failed to initialize database layer", slog.Any("error", err))
		os.Exit(1)
	}

	// 4. Initialize Clean Architecture database repositories
	userRepo := postgres.NewUserRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	txRepo := postgres.NewTransactionRepository(db)
	idempotencyRepo := postgres.NewIdempotencyRepository(db)
	auditRepo := postgres.NewAuditLogRepository(db)

	// 5. Initialize Clean Architecture business usecases
	// In production, load JWT secret from secure environment configs
	jwtSecret := "secpay-super-secure-jwt-signing-secret-key-12345"
	authUsecase := usecase.NewAuthUsecase(userRepo, auditRepo, jwtSecret)
	userUsecase := usecase.NewUserUsecase(userRepo)
	paymentUsecase := usecase.NewPaymentUsecase(accountRepo, txRepo)

	// 6. Initialize & Start Asynchronous Worker Pool
	workerPool := usecase.NewWorkerPool(paymentUsecase, idempotencyRepo, txRepo, auditRepo, 4, 100)
	workerPool.Start()
	defer workerPool.Stop()

	// 7. Initialize HTTP route handlers
	authHandler := handler.NewAuthHandler(authUsecase)
	kycHandler := handler.NewKYCHandler(userUsecase)
	paymentHandler := handler.NewPaymentHandler(workerPool, idempotencyRepo)

	// 8. Setup Gin routing engine with structured logging and rate limiting
	gin.SetMode(gin.ReleaseMode) // Set to release mode for production logging cleaner
	r := gin.New()
	
	// Register logging and recovery middlewares
	r.Use(middleware.StructuredLogger(), gin.Recovery())
	
	// Safeguard endpoints against DDoS and exhaustion via IP-based Token-Bucket Rate Limiter
	// Permits 10 requests/second with a burst allowance of 15 tokens per IP
	r.Use(middleware.RateLimiterMiddleware(10, 15))

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

	// 9. Start HTTP server
	slog.Info("SecPay HTTP server starting", slog.String("port", cfg.Port))
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Fatal: failed to start SecPay server: %v", err)
	}
}
