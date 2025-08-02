package api

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/auth"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/database"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scraper"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/services"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

// SetupRoutes configures all API routes
func SetupRoutes(r *gin.Engine, db *sql.DB, cfg *config.Config) error {
	// Wrap sql.DB in our database wrapper
	dbWrapper := &database.DB{DB: db}
	
	// Create services
	scraperService, err := scraper.NewService(dbWrapper, cfg, 5) // 5 concurrent scrapers
	if err != nil {
		return fmt.Errorf("failed to create scraper service: %w", err)
	}
	
	// Create centralized services
	services := services.NewServices(db, cfg)
	
	// Create handlers with proper service injection
	uploadHandler := NewUploadHandler(scraperService)
	authHandler := NewAuthHandler(db, cfg)            // Legacy handler
	authHandlerV2 := NewAuthHandlerV2(services.Auth)  // New service-based handler
	scoringHandler := NewScoringHandler(db)           // Legacy handler
	scoringHandlerV2 := NewScoringHandlerV2(services.Scoring) // New service-based handler
	pipelineHandler := NewPipelineHandler(db)         // TODO: Migrate to service layer
	leadsHandler := NewLeadsHandler(db, services.Scoring) // Using scoring service
	
	// Public routes
	public := r.Group("/api/v1")
	{
		// Use new service-based auth handlers
		public.POST("/auth/login", authHandlerV2.Login)
		public.POST("/auth/register", authHandlerV2.Register)
		public.POST("/auth/refresh", authHandlerV2.RefreshToken)
		public.POST("/auth/logout", authHandler.Logout)
	}
	
	// Protected routes
	protected := r.Group("/api/v1")
	protected.Use(auth.JWTMiddleware(cfg.JWTSecret))
	protected.Use(auth.CSRFMiddleware())
	{
		// CSV Upload endpoints
		protected.POST("/upload/csv", uploadHandler.UploadCSV)
		protected.GET("/jobs", uploadHandler.GetJobs)
		protected.GET("/jobs/:id", uploadHandler.GetJob)
		
		// Company endpoints
		protected.GET("/companies", uploadHandler.GetCompanies)
		protected.GET("/companies/:ticker", uploadHandler.GetCompany)
		
		// Health monitoring endpoints
		protected.GET("/health", uploadHandler.GetSystemHealth)
		protected.GET("/health/scraper", uploadHandler.GetScraperHealth)
		protected.POST("/health/scraper/reset", uploadHandler.ResetScraperHealth)
		
		// Scoring endpoints - using new service-based handlers
		protected.GET("/scoring/models", scoringHandlerV2.GetScoringModels)
		protected.GET("/scoring/models/:id", scoringHandlerV2.GetScoringModel)
		protected.POST("/scoring/models", scoringHandlerV2.CreateScoringModel)
		protected.PUT("/scoring/models/:id", scoringHandlerV2.UpdateScoringModel)
		protected.DELETE("/scoring/models/:id", scoringHandlerV2.DeleteScoringModel)
		
		// Company scoring endpoints
		protected.POST("/scoring/companies/:id/score", scoringHandlerV2.ScoreCompany)
		protected.GET("/scoring/companies/:id/scores", scoringHandlerV2.GetCompanyScores)
		protected.POST("/scoring/companies/:id/score/:model_id", scoringHandlerV2.ScoreCompanyWithModel)
		
		// Bulk scoring endpoints
		protected.POST("/scoring/models/:id/score-all", scoringHandlerV2.ScoreAllCompanies)
		
		// Automated pipeline endpoints
		protected.GET("/pipeline/status", pipelineHandler.GetPipelineStatus)
		protected.GET("/pipeline/config", pipelineHandler.GetPipelineConfig)
		protected.POST("/pipeline/start", pipelineHandler.StartPipeline)
		protected.POST("/pipeline/stop", pipelineHandler.StopPipeline)
		protected.POST("/pipeline/run-once", pipelineHandler.RunPipelineOnce)
		
		// Lead management endpoints
		protected.GET("/leads", leadsHandler.GetQualifiedLeads)
		protected.POST("/leads/export", leadsHandler.ExportQualifiedLeads)
		protected.GET("/leads/stats", leadsHandler.GetLeadStats)
		protected.GET("/leads/:ticker", leadsHandler.GetLeadByTicker)
	}
	
	return nil
}