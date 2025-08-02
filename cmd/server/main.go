package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/api"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/database"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/middleware"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize configuration
	cfg := config.New()

	// Initialize database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	
	// Initialize router
	r := gin.New()
	
	// Add security middleware
	r.Use(middleware.LoggingMiddleware())
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Use(middleware.CORSMiddleware(cfg))
	r.Use(middleware.InputValidationMiddleware())
	
	// Add rate limiting in production
	if cfg.EnableRateLimit {
		r.Use(middleware.RateLimitingMiddleware())
	}
	
	// Add recovery middleware
	r.Use(gin.Recovery())
	
	// Setup API routes
	if err := api.SetupRoutes(r, db, cfg); err != nil {
		log.Fatal("Failed to setup API routes:", err)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}