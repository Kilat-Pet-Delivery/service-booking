package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/auth"
	"github.com/Kilat-Pet-Delivery/lib-common/database"
	"github.com/Kilat-Pet-Delivery/lib-common/health"
	"github.com/Kilat-Pet-Delivery/lib-common/kafka"
	"github.com/Kilat-Pet-Delivery/lib-common/logger"
	"github.com/Kilat-Pet-Delivery/lib-common/middleware"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/application"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/config"
	bookingDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/booking"
	bookingEvents "github.com/Kilat-Pet-Delivery/service-booking/internal/events"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/handler"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.NewNamed(cfg.AppEnv, "service-booking")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	log.Info("starting service-booking",
		zap.String("port", cfg.Port),
	)

	// Connect to database
	dbConfig := database.PostgresConfig{
		Host:     cfg.DBConfig.Host,
		Port:     cfg.DBConfig.Port,
		User:     cfg.DBConfig.User,
		Password: cfg.DBConfig.Password,
		DBName:   cfg.DBConfig.DBName,
		SSLMode:  cfg.DBConfig.SSLMode,
	}
	db, err := database.Connect(dbConfig, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	// Run database migrations
	if cfg.AppEnv == "development" {
		if err := db.AutoMigrate(&repository.BookingModel{}, &repository.PetModel{}, &repository.PhotoModel{}); err != nil {
			log.Fatal("failed to run auto-migration", zap.Error(err))
		}
		log.Info("database migration completed (dev auto-migrate)")
	} else {
		dbURL := dbConfig.DatabaseURL()
		if err := database.RunMigrations(dbURL, "migrations", log); err != nil {
			log.Fatal("failed to run migrations", zap.Error(err))
		}
	}

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(
		cfg.JWTConfig.Secret,
		15*time.Minute,
		7*24*time.Hour,
	)

	// Initialize Kafka producer
	kafkaProducer := kafka.NewProducer(cfg.KafkaConfig.Brokers, log)
	defer func() { _ = kafkaProducer.Close() }()

	// Initialize repositories
	bookingRepo := repository.NewGormBookingRepository(db)
	petRepo := repository.NewGormPetRepository(db)

	// Initialize pricing strategy
	pricingStrategy := bookingDomain.NewStandardPricingStrategy()

	// Initialize application service
	bookingService := application.NewBookingService(
		bookingRepo,
		pricingStrategy,
		kafkaProducer,
		log,
	)

	// Initialize and start payment event consumer in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	groupID := cfg.KafkaConfig.GroupPrefix + "booking-service"
	paymentConsumer := bookingEvents.NewPaymentEventConsumer(
		cfg.KafkaConfig.Brokers,
		groupID,
		bookingService,
		log,
	)
	defer func() { _ = paymentConsumer.Close() }()

	go func() {
		log.Info("starting payment event consumer")
		if err := paymentConsumer.Start(ctx); err != nil && err != context.Canceled {
			log.Error("payment event consumer error", zap.Error(err))
		}
	}()

	// Initialize pet service
	petService := application.NewPetService(petRepo, log)

	// Initialize photo service
	photoRepo := repository.NewGormPhotoRepository(db)
	photoService := application.NewPhotoService(photoRepo, log)

	// Initialize HTTP handlers
	bookingHandler := handler.NewBookingHandler(bookingService)
	petHandler := handler.NewPetHandler(petService)
	photoHandler := handler.NewPhotoHandler(photoService)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Apply global middleware
	router.Use(middleware.RecoveryMiddleware(log))
	router.Use(middleware.LoggerMiddleware(log))
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.SecurityHeadersMiddleware())

	// Register health check routes
	healthHandler := health.NewHandler(db, "service-booking")
	healthHandler.RegisterRoutes(router)

	// Register routes
	bookingHandler.RegisterRoutes(&router.RouterGroup, jwtManager)
	petHandler.RegisterRoutes(&router.RouterGroup, jwtManager)
	photoHandler.RegisterRoutes(&router.RouterGroup, jwtManager)

	// Register admin handler routes
	adminBookingHandler := handler.NewAdminBookingHandler(bookingService)
	adminBookingHandler.RegisterRoutes(&router.RouterGroup, jwtManager)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("HTTP server starting", zap.String("addr", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down service-booking...")

	// Cancel the consumer context
	cancel()

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server forced shutdown", zap.Error(err))
	}

	log.Info("service-booking stopped")
}
