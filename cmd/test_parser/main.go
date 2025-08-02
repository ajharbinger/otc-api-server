package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scraper"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Load configuration
	cfg := &config.Config{
		OxyLabsUsername: os.Getenv("OXYLABS_USERNAME"),
		OxyLabsPassword: os.Getenv("OXYLABS_PASSWORD"),
		OxyLabsEndpoint: os.Getenv("OXYLABS_ENDPOINT"),
	}

	if cfg.OxyLabsUsername == "" || cfg.OxyLabsPassword == "" {
		log.Fatal("OxyLabs credentials not found in environment variables")
	}

	// Get ticker from command line or use ECGP as default
	ticker := "ECGP"
	if len(os.Args) > 1 {
		ticker = os.Args[1]
	}

	fmt.Printf("Testing parser with ticker: %s\n", ticker)
	fmt.Println("=====================================")

	// Create OxyLabs client and parser
	client := scraper.NewOxyLabsClient(cfg)
	parser := scraper.NewParser()
	defer client.Close()

	ctx := context.Background()

	// Test Overview page
	fmt.Println("\n1. Scraping Overview page...")
	overviewURL := fmt.Sprintf("https://www.otcmarkets.com/stock/%s/overview", ticker)
	
	overviewDoc, err := client.Get(ctx, overviewURL)
	if err != nil {
		log.Printf("Error scraping overview page: %v", err)
	} else {
		overviewData := parser.ParseOverviewPage(overviewDoc)
		fmt.Println("Overview Data:")
		printJSON(overviewData)
	}

	// Test Financials page
	fmt.Println("\n2. Scraping Financials page...")
	financialsURL := fmt.Sprintf("https://www.otcmarkets.com/stock/%s/financials", ticker)
	
	financialsDoc, err := client.Get(ctx, financialsURL)
	if err != nil {
		log.Printf("Error scraping financials page: %v", err)
	} else {
		financialsData := parser.ParseFinancialsPage(financialsDoc)
		fmt.Println("Financials Data:")
		printJSON(financialsData)
	}

	// Test Disclosure page
	fmt.Println("\n3. Scraping Disclosure page...")
	disclosureURL := fmt.Sprintf("https://www.otcmarkets.com/stock/%s/disclosure", ticker)
	
	disclosureDoc, err := client.Get(ctx, disclosureURL)
	if err != nil {
		log.Printf("Error scraping disclosure page: %v", err)
	} else {
		disclosureData := parser.ParseDisclosurePage(disclosureDoc)
		fmt.Println("Disclosure Data:")
		printJSON(disclosureData)
	}

	fmt.Println("\n=====================================")
	fmt.Println("Parser test completed!")
}

func printJSON(data map[string]interface{}) {
	if len(data) == 0 {
		fmt.Println("  No data extracted")
		return
	}

	jsonData, err := json.MarshalIndent(data, "  ", "  ")
	if err != nil {
		fmt.Printf("  Error formatting data: %v\n", err)
		return
	}
	fmt.Printf("  %s\n", string(jsonData))
}