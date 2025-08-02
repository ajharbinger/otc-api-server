package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

// ScoringPipeline handles automated scoring of companies
type ScoringPipeline struct {
	db             *sql.DB
	scoringService ScoringService
	engine         *scoring.ScoringEngine
	isRunning      bool
	stopChan       chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
}

// GetDB returns the database connection for health checks
func (sp *ScoringPipeline) GetDB() *sql.DB {
	return sp.db
}

// NewScoringPipeline creates a new automated scoring pipeline
func NewScoringPipeline(db *sql.DB) *ScoringPipeline {
	repos := repository.NewRepositories(db)
	return &ScoringPipeline{
		db:             db,
		scoringService: newScoringService(repos),
		engine:         scoring.NewScoringEngine(),
		stopChan:       make(chan struct{}),
	}
}

// PipelineConfig contains configuration for the scoring pipeline
type PipelineConfig struct {
	BatchSize           int           `json:"batch_size"`           // Number of companies to process at once
	IntervalMinutes     int           `json:"interval_minutes"`     // How often to run scoring (minutes)
	MaxConcurrent       int           `json:"max_concurrent"`       // Max concurrent scoring operations
	ProcessNewOnly      bool          `json:"process_new_only"`     // Only process companies never scored
	RescoreOlderThanDays int          `json:"rescore_older_than_days"` // Rescore companies older than X days
}

// DefaultPipelineConfig returns sensible defaults
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		BatchSize:           50,   // Process 50 companies at a time
		IntervalMinutes:     60,   // Run every hour
		MaxConcurrent:       10,   // 10 concurrent scoring operations
		ProcessNewOnly:      false, // Process all eligible companies
		RescoreOlderThanDays: 7,   // Rescore companies older than 1 week
	}
}

// Start begins the automated scoring pipeline
func (p *ScoringPipeline) Start(config PipelineConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return fmt.Errorf("pipeline is already running")
	}

	p.isRunning = true
	
	// Start the main pipeline loop
	p.wg.Add(1)
	go p.runPipeline(config)

	log.Printf("🎯 Scoring pipeline started with config: batch_size=%d, interval=%dm, max_concurrent=%d", 
		config.BatchSize, config.IntervalMinutes, config.MaxConcurrent)
	
	return nil
}

// Stop gracefully stops the scoring pipeline
func (p *ScoringPipeline) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return fmt.Errorf("pipeline is not running")
	}

	close(p.stopChan)
	p.wg.Wait()
	p.isRunning = false

	log.Println("🛑 Scoring pipeline stopped")
	return nil
}

// IsRunning returns whether the pipeline is currently running
func (p *ScoringPipeline) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isRunning
}

// RunOnce executes a single scoring cycle manually
func (p *ScoringPipeline) RunOnce(config PipelineConfig) (*PipelineStats, error) {
	ctx := context.Background()
	return p.executeScoringCycle(ctx, config)
}

// runPipeline is the main pipeline loop
func (p *ScoringPipeline) runPipeline(config PipelineConfig) {
	defer p.wg.Done()

	ticker := time.NewTicker(time.Duration(config.IntervalMinutes) * time.Minute)
	defer ticker.Stop()

	// Run immediately on start
	ctx := context.Background()
	if stats, err := p.executeScoringCycle(ctx, config); err != nil {
		log.Printf("❌ Initial scoring cycle failed: %v", err)
	} else {
		log.Printf("✅ Initial scoring cycle completed: %s", stats.Summary())
	}

	for {
		select {
		case <-p.stopChan:
			log.Println("📋 Pipeline stop signal received")
			return
		case <-ticker.C:
			if stats, err := p.executeScoringCycle(ctx, config); err != nil {
				log.Printf("❌ Scoring cycle failed: %v", err)
			} else {
				log.Printf("✅ Scoring cycle completed: %s", stats.Summary())
			}
		}
	}
}

// executeScoringCycle performs one complete scoring cycle
func (p *ScoringPipeline) executeScoringCycle(ctx context.Context, config PipelineConfig) (*PipelineStats, error) {
	startTime := time.Now()
	stats := &PipelineStats{
		StartTime: startTime,
		BatchSize: config.BatchSize,
	}

	log.Printf("🔄 Starting scoring cycle (batch_size=%d, max_concurrent=%d)", 
		config.BatchSize, config.MaxConcurrent)

	// Get companies that need scoring
	companies, err := p.getCompaniesForScoring(config)
	if err != nil {
		return stats, fmt.Errorf("failed to get companies for scoring: %w", err)
	}

	if len(companies) == 0 {
		stats.CompaniesProcessed = 0
		stats.EndTime = time.Now()
		log.Println("ℹ️  No companies need scoring at this time")
		return stats, nil
	}

	log.Printf("📊 Found %d companies that need scoring", len(companies))
	stats.CompaniesFound = len(companies)

	// Process companies in batches with concurrency control
	semaphore := make(chan struct{}, config.MaxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < len(companies); i += config.BatchSize {
		end := i + config.BatchSize
		if end > len(companies) {
			end = len(companies)
		}

		batch := companies[i:end]
		
		wg.Add(1)
		go func(companyBatch []CompanyForScoring) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process this batch
			batchStats := p.processBatch(ctx, companyBatch)
			
			// Update stats
			mu.Lock()
			stats.CompaniesProcessed += batchStats.Processed
			stats.CompaniesSucceeded += batchStats.Succeeded
			stats.CompaniesFailed += batchStats.Failed
			stats.ModelsApplied += batchStats.ModelsApplied
			mu.Unlock()

		}(batch)
	}

	wg.Wait()
	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	return stats, nil
}

// getCompaniesForScoring retrieves companies that need scoring
func (p *ScoringPipeline) getCompaniesForScoring(config PipelineConfig) ([]CompanyForScoring, error) {
	var query string
	var args []interface{}

	if config.ProcessNewOnly {
		// Only companies that have never been scored
		query = `
			SELECT c.id, c.ticker, c.company_name 
			FROM companies c
			LEFT JOIN company_scores cs ON c.id = cs.company_id
			WHERE cs.company_id IS NULL
			ORDER BY c.created_at DESC
			LIMIT $1
		`
		args = []interface{}{config.BatchSize * 10} // Get more than batch size for processing
	} else {
		// Companies never scored OR scored longer than rescore threshold ago
		rescoreDate := time.Now().AddDate(0, 0, -config.RescoreOlderThanDays)
		
		query = `
			WITH latest_scores AS (
				SELECT company_id, MAX(scored_at) as last_scored
				FROM company_scores
				GROUP BY company_id
			)
			SELECT c.id, c.ticker, c.company_name
			FROM companies c
			LEFT JOIN latest_scores ls ON c.id = ls.company_id
			WHERE ls.company_id IS NULL OR ls.last_scored < $1
			ORDER BY 
				CASE WHEN ls.company_id IS NULL THEN 0 ELSE 1 END,
				COALESCE(ls.last_scored, c.created_at) ASC
			LIMIT $2
		`
		args = []interface{}{rescoreDate, config.BatchSize * 10}
	}

	rows, err := p.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []CompanyForScoring
	for rows.Next() {
		var company CompanyForScoring
		if err := rows.Scan(&company.ID, &company.Ticker, &company.CompanyName); err != nil {
			return nil, err
		}
		companies = append(companies, company)
	}

	return companies, nil
}

// processBatch processes a batch of companies
func (p *ScoringPipeline) processBatch(ctx context.Context, companies []CompanyForScoring) BatchStats {
	stats := BatchStats{}
	
	for _, company := range companies {
		stats.Processed++
		
		// Score company against all active models
		if err := p.scoringService.ScoreCompany(company.ID); err != nil {
			log.Printf("❌ Failed to score company %s (%s): %v", company.Ticker, company.ID, err)
			stats.Failed++
		} else {
			log.Printf("✅ Scored company %s (%s)", company.Ticker, company.ID)
			stats.Succeeded++
			// Assuming 2 models (Double Black Diamond + Pink Market Opportunity)
			stats.ModelsApplied += 2
		}
	}

	return stats
}

// GetStats returns current pipeline statistics
func (p *ScoringPipeline) GetStats() (PipelineStatus, error) {
	status := PipelineStatus{
		IsRunning: p.IsRunning(),
		Timestamp: time.Now(),
	}

	// Get total companies and scored companies count
	var totalCompanies, scoredCompanies int
	
	if err := p.db.QueryRow("SELECT COUNT(*) FROM companies").Scan(&totalCompanies); err != nil {
		return status, err
	}
	
	if err := p.db.QueryRow("SELECT COUNT(DISTINCT company_id) FROM company_scores").Scan(&scoredCompanies); err != nil {
		return status, err
	}

	status.TotalCompanies = totalCompanies
	status.ScoredCompanies = scoredCompanies
	status.PendingCompanies = totalCompanies - scoredCompanies

	return status, nil
}

// Data structures

type CompanyForScoring struct {
	ID          string `json:"id"`
	Ticker      string `json:"ticker"`
	CompanyName string `json:"company_name"`
}

type BatchStats struct {
	Processed     int `json:"processed"`
	Succeeded     int `json:"succeeded"`
	Failed        int `json:"failed"`
	ModelsApplied int `json:"models_applied"`
}

type PipelineStats struct {
	StartTime           time.Time     `json:"start_time"`
	EndTime             time.Time     `json:"end_time"`
	Duration            time.Duration `json:"duration"`
	BatchSize           int           `json:"batch_size"`
	CompaniesFound      int           `json:"companies_found"`
	CompaniesProcessed  int           `json:"companies_processed"`
	CompaniesSucceeded  int           `json:"companies_succeeded"`
	CompaniesFailed     int           `json:"companies_failed"`
	ModelsApplied       int           `json:"models_applied"`
}

func (s *PipelineStats) Summary() string {
	return fmt.Sprintf("processed=%d, succeeded=%d, failed=%d, models_applied=%d, duration=%v",
		s.CompaniesProcessed, s.CompaniesSucceeded, s.CompaniesFailed, s.ModelsApplied, s.Duration.Round(time.Second))
}

type PipelineStatus struct {
	IsRunning        bool      `json:"is_running"`
	TotalCompanies   int       `json:"total_companies"`
	ScoredCompanies  int       `json:"scored_companies"`
	PendingCompanies int       `json:"pending_companies"`
	Timestamp        time.Time `json:"timestamp"`
}