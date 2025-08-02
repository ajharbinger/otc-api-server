package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/database"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scraper"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

func main() {
	// Command line flags
	ticker := flag.String("ticker", "AAPL", "Ticker symbol to scrape")
	healthOnly := flag.Bool("health", false, "Only run health check")
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Parse()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize configuration
	cfg := config.New()

	// Check if OxyLabs credentials are configured
	if !cfg.HasOxyLabsCredentials() {
		log.Fatal("OxyLabs credentials not configured. Please set OXYLABS_USERNAME and OXYLABS_PASSWORD")
	}

	fmt.Printf("Testing OxyLabs scraper with ticker: %s\n", *ticker)
	fmt.Printf("OxyLabs endpoint: %s\n", cfg.OxyLabsEndpoint)

	// Initialize database (only needed for full test, not health check)
	var db *database.DB
	var err error
	if !*healthOnly {
		db, err = database.New(cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		defer db.Close()
		
		fmt.Println("Database connection established")
	}

	// Initialize scraper service
	service, err := scraper.NewService(db, cfg, 5) // 5 concurrent workers
	if err != nil {
		log.Fatalf("Failed to initialize scraper service: %v", err)
	}
	defer service.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if *healthOnly {
		// Run health check only
		fmt.Println("Running health check...")
		if err := service.Health(ctx); err != nil {
			log.Fatalf("Health check failed: %v", err)
		}
		fmt.Println("✅ Health check passed! OxyLabs API is accessible")
		return
	}

	// Test single ticker scraping
	fmt.Printf("Scraping ticker: %s\n", *ticker)
	
	startTime := time.Now()
	company, err := service.ScrapeAndStore(ctx, *ticker)
	duration := time.Since(startTime)

	if err != nil {
		log.Fatalf("Failed to scrape ticker %s: %v", *ticker, err)
	}

	fmt.Printf("✅ Successfully scraped %s in %v\n", *ticker, duration)
	
	if *verbose {
		// Pretty print the company data
		companyJSON, err := json.MarshalIndent(company, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal company data: %v", err)
		} else {
			fmt.Println("\nScraped company data:")
			fmt.Println(string(companyJSON))
		}
	} else {
		// Print summary
		fmt.Printf("Company: %s\n", company.CompanyName)
		fmt.Printf("Market Tier: %s\n", company.MarketTier)
		fmt.Printf("Quote Status: %s\n", company.QuoteStatus)
		fmt.Printf("Website: %s\n", company.Website)
		fmt.Printf("Officers Count: %d\n", len(company.Officers))
		
		if company.LastFilingDate != nil {
			fmt.Printf("Last Filing: %s\n", company.LastFilingDate.Format("2006-01-02"))
		}
		if company.Last10KDate != nil {
			fmt.Printf("Last 10-K: %s\n", company.Last10KDate.Format("2006-01-02"))
		}
		if company.Last10QDate != nil {
			fmt.Printf("Last 10-Q: %s\n", company.Last10QDate.Format("2006-01-02"))
		}
	}

	fmt.Println("\n🎉 Test completed successfully!")
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Test the OxyLabs-based OTC Markets scraper\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample usage:\n")
		fmt.Fprintf(os.Stderr, "  %s -ticker=MSFT -v\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -health\n", os.Args[0])
	}
}