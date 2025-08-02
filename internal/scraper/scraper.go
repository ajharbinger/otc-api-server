package scraper

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

// Scraper orchestrates the scraping of OTC Markets data using OxyLabs
type Scraper struct {
	client         *OxyLabsClient
	parser         *Parser
	maxConcurrency int
	healthMonitor  *HealthMonitor
}

// New creates a new scraper instance with OxyLabs client
func New(cfg *config.Config, maxConcurrency int) (*Scraper, error) {
	if !cfg.HasOxyLabsCredentials() {
		return nil, fmt.Errorf("OxyLabs credentials are required")
	}

	return &Scraper{
		client:         NewOxyLabsClient(cfg),
		parser:         NewParser(),
		maxConcurrency: maxConcurrency,
		healthMonitor:  NewHealthMonitor(),
	}, nil
}

// ScrapeTicker scrapes all three pages for a single ticker using batch request
func (s *Scraper) ScrapeTicker(ctx context.Context, ticker string) (*models.ScrapedData, error) {
	scraped := &models.ScrapedData{
		Ticker:     ticker,
		ScrapedAt:  time.Now(),
		Overview:   make(map[string]interface{}),
		Financials: make(map[string]interface{}),
		Disclosure: make(map[string]interface{}),
		Errors:     []string{},
	}

	// URLs for the three pages
	urls := []string{
		fmt.Sprintf("https://www.otcmarkets.com/stock/%s/overview", ticker),
		fmt.Sprintf("https://www.otcmarkets.com/stock/%s/financials", ticker),
		fmt.Sprintf("https://www.otcmarkets.com/stock/%s/disclosure", ticker),
	}

	// Use batch request for efficiency
	docs, errors := s.client.GetBatch(ctx, urls)

	// Track overall success/failure
	hasErrors := false
	errorDetails := []string{}

	// Process overview page
	overviewURL := urls[0]
	if doc, exists := docs[overviewURL]; exists {
		scraped.Overview = s.parser.ParseOverviewPage(doc)
	} else if err, exists := errors[overviewURL]; exists {
		errorMsg := fmt.Sprintf("overview: %v", err)
		scraped.Errors = append(scraped.Errors, errorMsg)
		errorDetails = append(errorDetails, errorMsg)
		s.healthMonitor.RecordFailure(ticker, errorMsg, overviewURL)
		hasErrors = true
	}

	// Process financials page
	financialsURL := urls[1]
	if doc, exists := docs[financialsURL]; exists {
		scraped.Financials = s.parser.ParseFinancialsPage(doc)
	} else if err, exists := errors[financialsURL]; exists {
		errorMsg := fmt.Sprintf("financials: %v", err)
		scraped.Errors = append(scraped.Errors, errorMsg)
		errorDetails = append(errorDetails, errorMsg)
		s.healthMonitor.RecordFailure(ticker, errorMsg, financialsURL)
		hasErrors = true
	}

	// Process disclosure page
	disclosureURL := urls[2]
	if doc, exists := docs[disclosureURL]; exists {
		scraped.Disclosure = s.parser.ParseDisclosurePage(doc)
	} else if err, exists := errors[disclosureURL]; exists {
		errorMsg := fmt.Sprintf("disclosure: %v", err)
		scraped.Errors = append(scraped.Errors, errorMsg)
		errorDetails = append(errorDetails, errorMsg)
		s.healthMonitor.RecordFailure(ticker, errorMsg, disclosureURL)
		hasErrors = true
	}

	// Record overall success or failure
	if !hasErrors || len(docs) > 0 {
		// Consider partial success as success if at least one page was scraped
		s.healthMonitor.RecordSuccess(ticker)
	}

	// Log health status if there are issues
	if !s.healthMonitor.IsHealthy() {
		log.Printf("Scraper health warning for ticker %s: %v", ticker, errorDetails)
	}

	return scraped, nil
}

// ScrapeTickersBatch scrapes multiple tickers concurrently
func (s *Scraper) ScrapeTickersBatch(ctx context.Context, tickers []string, resultsChan chan<- *models.ScrapedData) error {
	defer close(resultsChan)

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, s.maxConcurrency)
	var wg sync.WaitGroup

	for _, ticker := range tickers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go func(t string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Scrape ticker
			scraped, err := s.ScrapeTicker(ctx, t)
			if err != nil {
				log.Printf("Error scraping ticker %s: %v", t, err)
				// Still send the result even if there were errors
				scraped = &models.ScrapedData{
					Ticker:     t,
					ScrapedAt:  time.Now(),
					Overview:   make(map[string]interface{}),
					Financials: make(map[string]interface{}),
					Disclosure: make(map[string]interface{}),
					Errors:     []string{err.Error()},
				}
			}

			select {
			case resultsChan <- scraped:
			case <-ctx.Done():
				return
			}
		}(ticker)
	}

	wg.Wait()
	return nil
}

// ScrapeTickersOptimized uses a more efficient approach for large batches
func (s *Scraper) ScrapeTickersOptimized(ctx context.Context, tickers []string, resultsChan chan<- *models.ScrapedData) error {
	defer close(resultsChan)

	// For large batches, we can group multiple tickers and use OxyLabs batch API more efficiently
	batchSize := 10 // Process 10 tickers at a time (30 URLs per batch)
	
	for i := 0; i < len(tickers); i += batchSize {
		end := i + batchSize
		if end > len(tickers) {
			end = len(tickers)
		}
		
		batch := tickers[i:end]
		if err := s.processBatch(ctx, batch, resultsChan); err != nil {
			log.Printf("Error processing batch %d-%d: %v", i, end, err)
		}
		
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// processBatch processes a batch of tickers efficiently
func (s *Scraper) processBatch(ctx context.Context, tickers []string, resultsChan chan<- *models.ScrapedData) error {
	// Build all URLs for the batch
	var allURLs []string
	tickerIndexMap := make(map[string]int) // URL to ticker index mapping
	
	for i, ticker := range tickers {
		urls := []string{
			fmt.Sprintf("https://www.otcmarkets.com/stock/%s/overview", ticker),
			fmt.Sprintf("https://www.otcmarkets.com/stock/%s/financials", ticker),
			fmt.Sprintf("https://www.otcmarkets.com/stock/%s/disclosure", ticker),
		}
		
		for j, url := range urls {
			allURLs = append(allURLs, url)
			tickerIndexMap[url] = i*3 + j // Store position info
		}
	}

	// Fetch all URLs in one batch request
	docs, errors := s.client.GetBatch(ctx, allURLs)

	// Process results for each ticker
	for _, ticker := range tickers {
		scraped := &models.ScrapedData{
			Ticker:     ticker,
			ScrapedAt:  time.Now(),
			Overview:   make(map[string]interface{}),
			Financials: make(map[string]interface{}),
			Disclosure: make(map[string]interface{}),
			Errors:     []string{},
		}

		// Get URLs for this ticker
		overviewURL := fmt.Sprintf("https://www.otcmarkets.com/stock/%s/overview", ticker)
		financialsURL := fmt.Sprintf("https://www.otcmarkets.com/stock/%s/financials", ticker)
		disclosureURL := fmt.Sprintf("https://www.otcmarkets.com/stock/%s/disclosure", ticker)

		// Process overview
		if doc, exists := docs[overviewURL]; exists {
			scraped.Overview = s.parser.ParseOverviewPage(doc)
		} else if err, exists := errors[overviewURL]; exists {
			scraped.Errors = append(scraped.Errors, fmt.Sprintf("overview: %v", err))
		}

		// Process financials
		if doc, exists := docs[financialsURL]; exists {
			scraped.Financials = s.parser.ParseFinancialsPage(doc)
		} else if err, exists := errors[financialsURL]; exists {
			scraped.Errors = append(scraped.Errors, fmt.Sprintf("financials: %v", err))
		}

		// Process disclosure
		if doc, exists := docs[disclosureURL]; exists {
			scraped.Disclosure = s.parser.ParseDisclosurePage(doc)
		} else if err, exists := errors[disclosureURL]; exists {
			scraped.Errors = append(scraped.Errors, fmt.Sprintf("disclosure: %v", err))
		}

		// Send result
		select {
		case resultsChan <- scraped:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// GetHealthStatus returns the current health status of the scraper
func (s *Scraper) GetHealthStatus() HealthStatus {
	return s.healthMonitor.GetHealthStatus()
}

// IsHealthy returns true if the scraper is operating within healthy parameters
func (s *Scraper) IsHealthy() bool {
	return s.healthMonitor.IsHealthy()
}

// GetFailureRate returns the current failure rate as a percentage
func (s *Scraper) GetFailureRate() float64 {
	return s.healthMonitor.GetFailureRate()
}

// ResetHealthMonitor clears all health monitoring data
func (s *Scraper) ResetHealthMonitor() {
	s.healthMonitor.Reset()
}

// Health performs a comprehensive health check
func (s *Scraper) Health(ctx context.Context) error {
	// First check OxyLabs client health
	if err := s.client.Health(ctx); err != nil {
		s.healthMonitor.RecordFailure("health_check", err.Error(), "")
		return fmt.Errorf("OxyLabs client health check failed: %w", err)
	}
	
	// Check overall scraper health status
	status := s.healthMonitor.GetHealthStatus()
	if !status.IsHealthy {
		return fmt.Errorf("scraper health check failed: %v", status.HealthIssues)
	}
	
	s.healthMonitor.RecordSuccess("health_check")
	return nil
}

// Close cleans up scraper resources
func (s *Scraper) Close() {
	s.client.Close()
}