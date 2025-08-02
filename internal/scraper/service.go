package scraper

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/database"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/services"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

// Service provides high-level scraping operations with OxyLabs integration
type Service struct {
	db             *database.DB
	scraper        *Scraper
	transformer    *Transformer
	cfg            *config.Config
	scoringService services.ScoringService
}

// NewService creates a new scraping service with OxyLabs support
func NewService(db *database.DB, cfg *config.Config, maxConcurrency int) (*Service, error) {
	scraper, err := New(cfg, maxConcurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize scraper: %w", err)
	}

	// Initialize scoring service for automatic scoring
	repos := repository.NewRepositories(db.DB)
	scoringService := services.NewScoringService(repos)

	return &Service{
		db:             db,
		scraper:        scraper,
		transformer:    NewTransformer(),
		cfg:            cfg,
		scoringService: scoringService,
	}, nil
}

// ScrapeAndStore scrapes a single ticker and stores it in the database
func (s *Service) ScrapeAndStore(ctx context.Context, ticker string) (*models.Company, error) {
	log.Printf("Starting scrape for ticker: %s", ticker)

	// Scrape the ticker using OxyLabs
	scraped, err := s.scraper.ScrapeTicker(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape ticker %s: %w", ticker, err)
	}

	// Log any scraping errors but continue processing
	if len(scraped.Errors) > 0 {
		log.Printf("Scraping warnings for ticker %s: %v", ticker, scraped.Errors)
	}

	// Transform to company model
	company, err := s.transformer.TransformToCompany(scraped)
	if err != nil {
		return nil, fmt.Errorf("failed to transform scraped data: %w", err)
	}

	// Validate data
	if validationErrors := s.transformer.ValidateCompanyData(company); len(validationErrors) > 0 {
		log.Printf("Validation warnings for ticker %s: %v", ticker, validationErrors)
	}

	// Store in database
	if err := s.storeCompany(ctx, company, scraped); err != nil {
		return nil, fmt.Errorf("failed to store company data: %w", err)
	}

	// Automatically score the company after storing
	if err := s.scoreCompanyAfterScrape(ctx, company.ID.String()); err != nil {
		log.Printf("Warning: Failed to score company %s after scraping: %v", ticker, err)
		// Don't fail the entire operation if scoring fails
	}

	log.Printf("Successfully scraped and stored ticker: %s", ticker)
	return company, nil
}

// ScrapeTickersBatch processes multiple tickers in a single job using optimized batching
func (s *Service) ScrapeTickersBatch(ctx context.Context, tickers []string, userID uuid.UUID, useOptimized bool) (*models.ScrapeJob, error) {
	log.Printf("Starting batch scrape for %d tickers", len(tickers))

	// Create scrape job record
	job := &models.ScrapeJob{
		ID:               uuid.New(),
		Status:           string(models.ScrapeJobPending),
		TotalTickers:     len(tickers),
		ProcessedTickers: 0,
		FailedTickers:    0,
		StartedBy:        userID,
		StartedAt:        time.Now(),
	}

	if err := s.createScrapeJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create scrape job: %w", err)
	}

	// Update job status to running
	job.Status = string(models.ScrapeJobRunning)
	if err := s.updateScrapeJob(ctx, job); err != nil {
		log.Printf("Failed to update job status to running: %v", err)
	}

	// Create results channel
	resultsChan := make(chan *models.ScrapedData, len(tickers))

	// Start scraping in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in scraping goroutine: %v", r)
				job.Status = string(models.ScrapeJobFailed)
				job.ErrorMessage = fmt.Sprintf("panic: %v", r)
			}
		}()

		var err error
		if useOptimized && len(tickers) > 10 {
			// Use optimized batch processing for large sets
			log.Printf("Using optimized batch processing for %d tickers", len(tickers))
			err = s.scraper.ScrapeTickersOptimized(ctx, tickers, resultsChan)
		} else {
			// Use standard concurrent processing
			log.Printf("Using standard concurrent processing for %d tickers", len(tickers))
			err = s.scraper.ScrapeTickersBatch(ctx, tickers, resultsChan)
		}

		if err != nil {
			log.Printf("Error in batch scraping: %v", err)
			job.Status = string(models.ScrapeJobFailed)
			job.ErrorMessage = err.Error()
		} else {
			job.Status = string(models.ScrapeJobCompleted)
		}

		// Process results
		processedCount := 0
		failedCount := 0

		for scraped := range resultsChan {
			if company, err := s.transformer.TransformToCompany(scraped); err != nil {
				log.Printf("Failed to transform ticker %s: %v", scraped.Ticker, err)
				failedCount++
			} else {
				if err := s.storeCompany(ctx, company, scraped); err != nil {
					log.Printf("Failed to store ticker %s: %v", scraped.Ticker, err)
					failedCount++
				} else {
					processedCount++
					// Automatically score the company after storing (non-blocking)
					go func(companyID string, ticker string) {
						if err := s.scoreCompanyAfterScrape(ctx, companyID); err != nil {
							log.Printf("Warning: Failed to score company %s after batch scraping: %v", ticker, err)
						}
					}(company.ID.String(), scraped.Ticker)
				}
			}

			// Update job progress periodically
			if (processedCount+failedCount)%10 == 0 {
				job.ProcessedTickers = processedCount
				job.FailedTickers = failedCount
				if err := s.updateScrapeJob(ctx, job); err != nil {
					log.Printf("Failed to update job progress: %v", err)
				}
			}
		}

		// Update final job status
		job.ProcessedTickers = processedCount
		job.FailedTickers = failedCount
		completedAt := time.Now()
		job.CompletedAt = &completedAt

		if err := s.updateScrapeJob(ctx, job); err != nil {
			log.Printf("Failed to update final job status: %v", err)
		}

		log.Printf("Batch scrape completed. Processed: %d, Failed: %d", processedCount, failedCount)
	}()

	return job, nil
}

// ScrapeTickerSingle is a convenience method for single ticker scraping
func (s *Service) ScrapeTickerSingle(ctx context.Context, ticker string, userID uuid.UUID) (*models.ScrapeJob, error) {
	return s.ScrapeTickersBatch(ctx, []string{ticker}, userID, false)
}

// storeCompany stores company data and historical snapshot with better error handling
func (s *Service) storeCompany(ctx context.Context, company *models.Company, scraped *models.ScrapedData) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if company exists
	var existingID uuid.UUID
	var existingUpdatedAt time.Time
	err = tx.QueryRowContext(ctx,
		"SELECT id, updated_at FROM companies WHERE ticker = $1",
		company.Ticker,
	).Scan(&existingID, &existingUpdatedAt)

	if err == sql.ErrNoRows {
		// Insert new company
		company.ID = uuid.New()
		company.CreatedAt = time.Now()
		
		_, err = tx.ExecContext(ctx, `
			INSERT INTO companies (
				id, ticker, company_name, market_tier, quote_status, trading_volume,
				website, description, officers, address, transfer_agent, auditor,
				last_10k_date, last_10q_date, last_filing_date, profile_verified,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
			company.ID, company.Ticker, company.CompanyName, company.MarketTier,
			company.QuoteStatus, company.TradingVolume, company.Website,
			company.Description, company.Officers, company.Address,
			company.TransferAgent, company.Auditor, company.Last10KDate,
			company.Last10QDate, company.LastFilingDate, company.ProfileVerified,
			company.CreatedAt, company.UpdatedAt,
		)
		
		if err != nil {
			return fmt.Errorf("failed to insert new company: %w", err)
		}
		
		log.Printf("Inserted new company: %s", company.Ticker)
	} else if err == nil {
		// Update existing company
		company.ID = existingID
		company.CreatedAt = existingUpdatedAt // Preserve original creation time
		
		_, err = tx.ExecContext(ctx, `
			UPDATE companies SET
				company_name = $2, market_tier = $3, quote_status = $4, trading_volume = $5,
				website = $6, description = $7, officers = $8, address = $9,
				transfer_agent = $10, auditor = $11, last_10k_date = $12, last_10q_date = $13,
				last_filing_date = $14, profile_verified = $15, updated_at = $16
			WHERE id = $1`,
			company.ID, company.CompanyName, company.MarketTier, company.QuoteStatus,
			company.TradingVolume, company.Website, company.Description,
			company.Officers, company.Address, company.TransferAgent, company.Auditor,
			company.Last10KDate, company.Last10QDate, company.LastFilingDate,
			company.ProfileVerified, company.UpdatedAt,
		)
		
		if err != nil {
			return fmt.Errorf("failed to update existing company: %w", err)
		}
		
		log.Printf("Updated existing company: %s", company.Ticker)
	} else {
		return fmt.Errorf("failed to check existing company: %w", err)
	}

	// Store historical snapshot
	snapshotData := map[string]interface{}{
		"scraped_data": scraped,
		"company_data": company,
		"scrape_metadata": map[string]interface{}{
			"oxylabs_used": true,
			"errors": scraped.Errors,
		},
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO company_history (company_id, snapshot_data, scraped_at)
		VALUES ($1, $2, $3)`,
		company.ID, snapshotData, scraped.ScrapedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to store historical snapshot: %w", err)
	}

	return tx.Commit()
}

// createScrapeJob creates a new scrape job record
func (s *Service) createScrapeJob(ctx context.Context, job *models.ScrapeJob) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scrape_jobs (
			id, status, total_tickers, processed_tickers, failed_tickers,
			started_by, started_at, completed_at, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		job.ID, job.Status, job.TotalTickers, job.ProcessedTickers,
		job.FailedTickers, job.StartedBy, job.StartedAt, job.CompletedAt,
		job.ErrorMessage,
	)
	return err
}

// updateScrapeJob updates an existing scrape job
func (s *Service) updateScrapeJob(ctx context.Context, job *models.ScrapeJob) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE scrape_jobs SET
			status = $2, processed_tickers = $3, failed_tickers = $4,
			completed_at = $5, error_message = $6
		WHERE id = $1`,
		job.ID, job.Status, job.ProcessedTickers, job.FailedTickers,
		job.CompletedAt, job.ErrorMessage,
	)
	return err
}

// GetUserJobs retrieves all scrape jobs for a specific user
func (s *Service) GetUserJobs(ctx context.Context, userID uuid.UUID) ([]*models.ScrapeJob, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, status, total_tickers, processed_tickers, failed_tickers,
			   started_by, started_at, completed_at, error_message
		FROM scrape_jobs 
		WHERE started_by = $1
		ORDER BY started_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.ScrapeJob
	for rows.Next() {
		job := &models.ScrapeJob{}
		err := rows.Scan(
			&job.ID, &job.Status, &job.TotalTickers, &job.ProcessedTickers,
			&job.FailedTickers, &job.StartedBy, &job.StartedAt,
			&job.CompletedAt, &job.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetJob retrieves a scrape job by ID (alias for GetScrapeJob for API compatibility)
func (s *Service) GetJob(ctx context.Context, jobID uuid.UUID) (*models.ScrapeJob, error) {
	return s.GetScrapeJob(ctx, jobID)
}

// GetScrapeJob retrieves a scrape job by ID
func (s *Service) GetScrapeJob(ctx context.Context, jobID uuid.UUID) (*models.ScrapeJob, error) {
	job := &models.ScrapeJob{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, status, total_tickers, processed_tickers, failed_tickers,
			   started_by, started_at, completed_at, error_message
		FROM scrape_jobs WHERE id = $1`,
		jobID,
	).Scan(
		&job.ID, &job.Status, &job.TotalTickers, &job.ProcessedTickers,
		&job.FailedTickers, &job.StartedBy, &job.StartedAt,
		&job.CompletedAt, &job.ErrorMessage,
	)

	if err != nil {
		return nil, err
	}

	return job, nil
}

// GetRecentScrapeJobs retrieves recent scrape jobs
func (s *Service) GetRecentScrapeJobs(ctx context.Context, limit int) ([]*models.ScrapeJob, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, status, total_tickers, processed_tickers, failed_tickers,
			   started_by, started_at, completed_at, error_message
		FROM scrape_jobs 
		ORDER BY started_at DESC 
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.ScrapeJob
	for rows.Next() {
		job := &models.ScrapeJob{}
		err := rows.Scan(
			&job.ID, &job.Status, &job.TotalTickers, &job.ProcessedTickers,
			&job.FailedTickers, &job.StartedBy, &job.StartedAt,
			&job.CompletedAt, &job.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetScraperHealthStatus returns detailed health status of the scraper
func (s *Service) GetScraperHealthStatus() HealthStatus {
	return s.scraper.GetHealthStatus()
}

// IsScraperHealthy returns true if the scraper is healthy
func (s *Service) IsScraperHealthy() bool {
	return s.scraper.IsHealthy()
}

// GetScraperFailureRate returns the current scraper failure rate
func (s *Service) GetScraperFailureRate() float64 {
	return s.scraper.GetFailureRate()
}

// ResetScraperHealthMonitor resets the health monitor
func (s *Service) ResetScraperHealthMonitor() {
	s.scraper.ResetHealthMonitor()
}

// Health checks the health of all scraping components
func (s *Service) Health(ctx context.Context) error {
	// Check OxyLabs connectivity and scraper health
	if err := s.scraper.Health(ctx); err != nil {
		return fmt.Errorf("scraper health check failed: %w", err)
	}

	// Check database connectivity
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}

// GetCompanies retrieves paginated company data with filtering
func (s *Service) GetCompanies(ctx context.Context, page, limit int, search, marketTier string) ([]models.Company, int, error) {
	offset := (page - 1) * limit
	
	// Build query with filters
	baseQuery := `SELECT id, ticker, company_name, market_tier, quote_status, trading_volume,
	              website, description, officers, address, transfer_agent, auditor,
	              last_10k_date, last_10q_date, last_filing_date, profile_verified,
	              created_at, updated_at FROM companies`
	
	countQuery := `SELECT COUNT(*) FROM companies`
	
	var conditions []string
	var args []interface{}
	argIndex := 1
	
	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(ticker ILIKE $%d OR company_name ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+search+"%")
		argIndex++
	}
	
	if marketTier != "" {
		conditions = append(conditions, fmt.Sprintf("market_tier = $%d", argIndex))
		args = append(args, marketTier)
		argIndex++
	}
	
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}
	
	// Get total count
	var total int
	err := s.db.QueryRowContext(ctx, countQuery+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get company count: %w", err)
	}
	
	// Get companies with pagination
	finalQuery := baseQuery + whereClause + fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)
	
	rows, err := s.db.QueryContext(ctx, finalQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query companies: %w", err)
	}
	defer rows.Close()
	
	var companies []models.Company
	for rows.Next() {
		var company models.Company
		err := rows.Scan(
			&company.ID, &company.Ticker, &company.CompanyName, &company.MarketTier,
			&company.QuoteStatus, &company.TradingVolume, &company.Website,
			&company.Description, &company.Officers, &company.Address,
			&company.TransferAgent, &company.Auditor, &company.Last10KDate,
			&company.Last10QDate, &company.LastFilingDate, &company.ProfileVerified,
			&company.CreatedAt, &company.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan company: %w", err)
		}
		companies = append(companies, company)
	}
	
	return companies, total, nil
}

// GetCompanyByTicker retrieves a specific company by ticker
func (s *Service) GetCompanyByTicker(ctx context.Context, ticker string) (*models.Company, error) {
	query := `SELECT id, ticker, company_name, market_tier, quote_status, trading_volume,
	          website, description, officers, address, transfer_agent, auditor,
	          last_10k_date, last_10q_date, last_filing_date, profile_verified,
	          created_at, updated_at FROM companies WHERE ticker = $1`
	
	var company models.Company
	err := s.db.QueryRowContext(ctx, query, strings.ToUpper(ticker)).Scan(
		&company.ID, &company.Ticker, &company.CompanyName, &company.MarketTier,
		&company.QuoteStatus, &company.TradingVolume, &company.Website,
		&company.Description, &company.Officers, &company.Address,
		&company.TransferAgent, &company.Auditor, &company.Last10KDate,
		&company.Last10QDate, &company.LastFilingDate, &company.ProfileVerified,
		&company.CreatedAt, &company.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("company with ticker %s not found", ticker)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to get company: %w", err)
	}
	
	return &company, nil
}

// scoreCompanyAfterScrape automatically scores a company after scraping using all active ICP models
func (s *Service) scoreCompanyAfterScrape(ctx context.Context, companyID string) error {
	log.Printf("Starting automatic scoring for company ID: %s", companyID)
	
	// Create context with timeout for scoring operation
	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	// Score the company using the scoring service
	if err := s.scoringService.ScoreCompany(companyID); err != nil {
		return fmt.Errorf("failed to score company %s: %w", companyID, err)
	}
	
	log.Printf("Successfully scored company ID: %s", companyID)
	return nil
}

// GetScoringService returns the scoring service instance for external use
func (s *Service) GetScoringService() services.ScoringService {
	return s.scoringService
}

// ScoreExistingCompanies scores all existing companies that haven't been scored yet
func (s *Service) ScoreExistingCompanies(ctx context.Context, modelID string) error {
	log.Printf("Starting bulk scoring of existing companies with model: %s", modelID)
	
	// Get all company IDs from database
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM companies ORDER BY updated_at DESC")
	if err != nil {
		return fmt.Errorf("failed to get company IDs: %w", err)
	}
	defer rows.Close()
	
	var companyIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan company ID: %w", err)
		}
		companyIDs = append(companyIDs, id)
	}
	
	log.Printf("Found %d companies to score", len(companyIDs))
	
	// Score companies in batches to avoid overwhelming the system
	batchSize := 10
	for i := 0; i < len(companyIDs); i += batchSize {
		end := i + batchSize
		if end > len(companyIDs) {
			end = len(companyIDs)
		}
		
		batch := companyIDs[i:end]
		for _, companyID := range batch {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if modelID != "" {
					// Score with specific model
					if _, err := s.scoringService.ScoreCompanyWithModel(companyID, modelID); err != nil {
						log.Printf("Failed to score company %s with model %s: %v", companyID, modelID, err)
					}
				} else {
					// Score with all active models
					if err := s.scoringService.ScoreCompany(companyID); err != nil {
						log.Printf("Failed to score company %s: %v", companyID, err)
					}
				}
			}
		}
		
		// Small delay between batches to avoid overloading
		time.Sleep(100 * time.Millisecond)
	}
	
	log.Printf("Completed bulk scoring of %d companies", len(companyIDs))
	return nil
}

// Close cleans up service resources
func (s *Service) Close() {
	s.scraper.Close()
}