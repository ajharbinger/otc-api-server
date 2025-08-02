package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/database"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/services"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

var (
	pipeline       *services.ScoringPipeline
	pipelineMutex  sync.RWMutex
	lastHealthy    time.Time
	isHealthy      bool
)

func main() {
	fmt.Println("🚂 OTC Markets Railway Scoring Pipeline")
	fmt.Println("=======================================")

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize configuration
	cfg := config.New()

	// Validate required environment variables
	if cfg.DatabaseURL == "" {
		log.Println("❌ ERROR: DATABASE_URL environment variable is not set!")
		log.Println("")
		log.Println("To fix this:")
		log.Println("1. Go to your Railway dashboard")
		log.Println("2. Click on your service")
		log.Println("3. Go to Variables tab")
		log.Println("4. Add DATABASE_URL with your PostgreSQL connection string")
		log.Println("")
		log.Println("Example format:")
		log.Println("DATABASE_URL=postgres://user:password@host:5432/database?sslmode=require")
		log.Fatal("Cannot start without database connection")
	}

	// Check for other required variables
	missingVars := []string{}
	if cfg.OxyLabsUsername == "" {
		missingVars = append(missingVars, "OXYLABS_USERNAME")
	}
	if cfg.OxyLabsPassword == "" {
		missingVars = append(missingVars, "OXYLABS_PASSWORD")
	}
	if cfg.JWTSecret == "" {
		missingVars = append(missingVars, "JWT_SECRET")
	}

	if len(missingVars) > 0 {
		log.Println("⚠️  WARNING: Missing required environment variables:")
		for _, v := range missingVars {
			log.Printf("   - %s", v)
		}
		log.Println("")
		log.Println("Add these in Railway dashboard → Variables tab")
		log.Println("The app may not function correctly without them.")
		log.Println("")
	}

	// Get port for Railway
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start health check server (required by Railway)
	go startHealthServer(port)

	// Initialize database connection
	log.Printf("Connecting to database...")
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✅ Database connection established")

	// Create scoring pipeline  
	pipeline = services.NewScoringPipeline(db.DB)

	// Parse pipeline configuration
	pipelineConfig := parsePipelineConfig()
	
	fmt.Printf("📋 Pipeline Configuration:\n")
	fmt.Printf("   • Batch Size: %d companies\n", pipelineConfig.BatchSize)
	fmt.Printf("   • Interval: %d minutes\n", pipelineConfig.IntervalMinutes)
	fmt.Printf("   • Max Concurrent: %d operations\n", pipelineConfig.MaxConcurrent)
	fmt.Printf("   • Process New Only: %v\n", pipelineConfig.ProcessNewOnly)
	fmt.Printf("   • Rescore After: %d days\n", pipelineConfig.RescoreOlderThanDays)

	// Start the pipeline
	if err := pipeline.Start(pipelineConfig); err != nil {
		log.Fatalf("❌ Failed to start pipeline: %v", err)
	}

	// Mark as healthy
	pipelineMutex.Lock()
	isHealthy = true
	lastHealthy = time.Now()
	pipelineMutex.Unlock()

	fmt.Printf("\n🚀 Railway scoring pipeline started on port %s\n", port)
	fmt.Println("Health check available at /health")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Keep the service running
	fmt.Println("\n🔄 Pipeline is running...")
	fmt.Println("Press Ctrl+C to stop gracefully")

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutdown signal received, stopping pipeline...")

	// Mark as unhealthy
	pipelineMutex.Lock()
	isHealthy = false
	pipelineMutex.Unlock()

	// Stop the pipeline gracefully
	if err := pipeline.Stop(); err != nil {
		log.Printf("❌ Error stopping pipeline: %v", err)
	} else {
		fmt.Println("✅ Pipeline stopped successfully")
	}
}

func startHealthServer(port string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", securityMiddleware(healthHandler))
	mux.HandleFunc("/", securityMiddleware(rootHandler))
	mux.HandleFunc("/status", securityMiddleware(statusHandler))
	mux.HandleFunc("/metrics", securityMiddleware(metricsHandler))
	mux.HandleFunc("/ready", securityMiddleware(readinessHandler))
	
	log.Printf("Starting health server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Health server failed: %v", err)
	}
}

func securityMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Log incoming request
		log.Printf("[%s] %s %s - %s", time.Now().Format("2006-01-02 15:04:05"), r.Method, r.URL.Path, r.RemoteAddr)
		
		// Security headers
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Content Security Policy for health endpoints
		csp := "default-src 'none'; " +
			"script-src 'none'; " +
			"style-src 'none'; " +
			"img-src 'none'; " +
			"connect-src 'self'; " +
			"font-src 'none'; " +
			"object-src 'none'; " +
			"media-src 'none'; " +
			"frame-src 'none'; " +
			"base-uri 'none'; " +
			"form-action 'none'"
		w.Header().Set("Content-Security-Policy", csp)
		
		// Restricted CORS headers for health checks only
		origin := r.Header.Get("Origin")
		allowedOrigins := getAllowedHealthCheckOrigins()
		
		isAllowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				isAllowed = true
				break
			}
		}
		
		if isAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")
		
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// Wrap response writer to capture status code
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next(wrappedWriter, r)
		
		// Log response
		duration := time.Since(start)
		log.Printf("[%s] %s %s - %d (%v)", time.Now().Format("2006-01-02 15:04:05"), r.Method, r.URL.Path, wrappedWriter.statusCode, duration)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	pipelineMutex.RLock()
	healthy := isHealthy
	lastCheck := lastHealthy
	pipelineMutex.RUnlock()

	// Set cache-control headers to prevent caching
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")

	if healthy && time.Since(lastCheck) < 5*time.Minute {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "healthy", "service": "scoring-pipeline", "last_check": "%s"}`, lastCheck.Format(time.RFC3339))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status": "unhealthy", "service": "scoring-pipeline", "last_check": "%s"}`, lastCheck.Format(time.RFC3339))
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	// Set cache-control headers to prevent caching
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"service": "OTC Markets Scoring Pipeline",
		"version": "1.0.0",
		"status": "running",
		"platform": "Railway",
		"endpoints": {
			"health": "/health",
			"status": "/status"
		}
	}`)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	// Set cache-control headers to prevent caching
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")

	if pipeline == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"error": "pipeline not initialized"}`)
		return
	}

	status, err := pipeline.GetStats()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "failed to get pipeline stats: %s"}`, err.Error())
		return
	}
	fmt.Fprintf(w, `{
		"pipeline_running": %v,
		"total_companies": %d,
		"scored_companies": %d,
		"pending_companies": %d,
		"timestamp": "%s"
	}`, status.IsRunning, status.TotalCompanies, status.ScoredCompanies, status.PendingCompanies, status.Timestamp.Format(time.RFC3339))
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	pipelineMutex.RLock()
	healthy := isHealthy
	lastCheck := lastHealthy
	pipelineMutex.RUnlock()

	// Get database stats if available
	var dbStats string
	if pipeline != nil && pipeline.GetDB() != nil {
		stats := pipeline.GetDB().Stats()
		dbStats = fmt.Sprintf(`"db_open_connections": %d, "db_idle_connections": %d, "db_in_use": %d, "db_wait_count": %d`,
			stats.OpenConnections, stats.Idle, stats.InUse, stats.WaitCount)
	} else {
		dbStats = `"db_open_connections": 0, "db_idle_connections": 0, "db_in_use": 0, "db_wait_count": 0`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{
		"service": "scoring-pipeline",
		"healthy": %t,
		"last_health_check": "%s",
		"uptime_seconds": %.0f,
		%s,
		"timestamp": "%s"
	}`, healthy, lastCheck.Format(time.RFC3339), time.Since(lastCheck).Seconds(), dbStats, time.Now().Format(time.RFC3339))
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	pipelineMutex.RLock()
	ready := pipeline != nil && isHealthy && time.Since(lastHealthy) < 10*time.Minute
	pipelineMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	
	if ready {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"ready": true, "service": "scoring-pipeline", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"ready": false, "service": "scoring-pipeline", "reason": "pipeline not ready", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	}
}

func parsePipelineConfig() services.PipelineConfig {
	config := services.DefaultPipelineConfig()

	// Override with environment variables if present
	if val := os.Getenv("PIPELINE_BATCH_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.BatchSize = parsed
		}
	}

	if val := os.Getenv("PIPELINE_INTERVAL_MINUTES"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.IntervalMinutes = parsed
		}
	}

	if val := os.Getenv("PIPELINE_MAX_CONCURRENT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxConcurrent = parsed
		}
	}

	if val := os.Getenv("PIPELINE_PROCESS_NEW_ONLY"); val != "" {
		config.ProcessNewOnly = val == "true"
	}

	if val := os.Getenv("PIPELINE_RESCORE_OLDER_THAN_DAYS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.RescoreOlderThanDays = parsed
		}
	}

	return config
}

// getAllowedHealthCheckOrigins returns allowed origins for health check endpoints
func getAllowedHealthCheckOrigins() []string {
	// Get from environment variable or use defaults
	origins := os.Getenv("HEALTH_ALLOWED_ORIGINS")
	if origins != "" {
		return strings.Split(origins, ",")
	}
	
	// Default allowed origins for health checks
	// Update these for your production monitoring services
	return []string{
		"https://railway.app",
		"https://your-monitoring-service.com",
		"https://your-production-domain.com",
	}
}