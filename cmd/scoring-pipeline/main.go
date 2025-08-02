package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/database"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/services"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

func main() {
	fmt.Println("🎯 OTC Markets Automated Scoring Pipeline")
	fmt.Println("==========================================")

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

	// Create scoring pipeline
	pipeline := services.NewScoringPipeline(db)

	// Parse configuration from environment or use defaults
	pipelineConfig := parsePipelineConfig()

	fmt.Printf("📋 Pipeline Configuration:\n")
	fmt.Printf("   • Batch Size: %d companies\n", pipelineConfig.BatchSize)
	fmt.Printf("   • Interval: %d minutes\n", pipelineConfig.IntervalMinutes)
	fmt.Printf("   • Max Concurrent: %d operations\n", pipelineConfig.MaxConcurrent)
	fmt.Printf("   • Process New Only: %v\n", pipelineConfig.ProcessNewOnly)
	fmt.Printf("   • Rescore After: %d days\n", pipelineConfig.RescoreOlderThanDays)

	// Check if this is a one-time run
	if len(os.Args) > 1 && os.Args[1] == "--once" {
		fmt.Println("\n🔄 Running one-time scoring cycle...")
		stats, err := pipeline.RunOnce(pipelineConfig)
		if err != nil {
			log.Fatalf("❌ One-time scoring failed: %v", err)
		}
		
		fmt.Printf("\n✅ One-time scoring completed!\n")
		fmt.Printf("   • Duration: %v\n", stats.Duration.Round(time.Second))
		fmt.Printf("   • Companies Found: %d\n", stats.CompaniesFound)
		fmt.Printf("   • Companies Processed: %d\n", stats.CompaniesProcessed)
		fmt.Printf("   • Companies Succeeded: %d\n", stats.CompaniesSucceeded)
		fmt.Printf("   • Companies Failed: %d\n", stats.CompaniesFailed)
		fmt.Printf("   • Models Applied: %d\n", stats.ModelsApplied)
		return
	}

	// Start the automated pipeline
	if err := pipeline.Start(pipelineConfig); err != nil {
		log.Fatalf("❌ Failed to start pipeline: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\n🚀 Automated scoring pipeline is running...")
	fmt.Println("Press Ctrl+C to stop gracefully")

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutdown signal received, stopping pipeline...")

	// Stop the pipeline gracefully
	if err := pipeline.Stop(); err != nil {
		log.Printf("❌ Error stopping pipeline: %v", err)
	} else {
		fmt.Println("✅ Pipeline stopped successfully")
	}
}

// parsePipelineConfig parses pipeline configuration from environment variables
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