package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/auth"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scraper"
)

// UploadHandler handles CSV upload and scraping operations
type UploadHandler struct {
	scraperService *scraper.Service
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(scraperService *scraper.Service) *UploadHandler {
	return &UploadHandler{
		scraperService: scraperService,
	}
}

// UploadCSVRequest represents the CSV upload request
type UploadCSVRequest struct {
	UseOptimized bool `json:"use_optimized" form:"use_optimized"`
}

// UploadCSV handles CSV file upload and queues scraping job
func (h *UploadHandler) UploadCSV(c *gin.Context) {
	// Create a context with timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Parse form data
	var req UploadCSVRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("csv_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No CSV file provided"})
		return
	}
	defer file.Close()

	// Validate file type
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be a CSV"})
		return
	}

	// Parse CSV content
	tickers, err := h.parseCSV(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse CSV: %v", err)})
		return
	}

	if len(tickers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV file contains no valid tickers"})
		return
	}

	// Validate ticker count (prevent overload)
	if len(tickers) > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many tickers. Maximum 10,000 allowed per upload"})
		return
	}

	// Get user ID from JWT token
	userID, exists := c.Get(auth.UserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Create and start scraping job with transaction safety
	job, err := h.scraperService.ScrapeTickersBatch(ctx, tickers, userUUID, req.UseOptimized)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start scraping job: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "CSV upload successful, scraping job started",
		"job_id":        job.ID,
		"total_tickers": len(tickers),
		"status":        job.Status,
		"filename":      header.Filename,
	})
}

// parseCSV extracts tickers from CSV file
func (h *UploadHandler) parseCSV(file io.Reader) ([]string, error) {
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	
	var tickers []string
	tickerSet := make(map[string]bool) // Use set to avoid duplicates
	
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	// Process each row
	for i, record := range records {
		if len(record) == 0 {
			continue
		}

		// Get ticker from first column, trim whitespace and convert to uppercase
		ticker := strings.TrimSpace(strings.ToUpper(record[0]))
		
		// Skip empty rows or potential headers
		if ticker == "" || ticker == "TICKER" || ticker == "SYMBOL" {
			continue
		}

		// Validate ticker format (basic validation)
		if !h.isValidTicker(ticker) {
			return nil, fmt.Errorf("invalid ticker format '%s' on line %d", ticker, i+1)
		}

		// Add to set (avoids duplicates)
		if !tickerSet[ticker] {
			tickerSet[ticker] = true
			tickers = append(tickers, ticker)
		}
	}

	return tickers, nil
}

// isValidTicker performs basic ticker validation
func (h *UploadHandler) isValidTicker(ticker string) bool {
	// Basic validation: 1-10 characters, alphanumeric only
	if len(ticker) < 1 || len(ticker) > 10 {
		return false
	}
	
	for _, char := range ticker {
		if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return false
		}
	}
	
	return true
}

// GetJobs returns all scraping jobs for the authenticated user
func (h *UploadHandler) GetJobs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user ID from JWT token
	userID, exists := c.Get(auth.UserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	jobs, err := h.scraperService.GetUserJobs(ctx, userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch jobs: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

// GetJob returns a specific scraping job
func (h *UploadHandler) GetJob(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jobIDStr := c.Param("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID format"})
		return
	}

	job, err := h.scraperService.GetJob(ctx, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"job": job})
}

// GetCompanies returns paginated company data
func (h *UploadHandler) GetCompanies(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Parse query parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")
	search := c.Query("search")
	marketTier := c.Query("market_tier")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	// Get companies from service
	companies, total, err := h.scraperService.GetCompanies(ctx, page, limit, search, marketTier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch companies: %v", err)})
		return
	}

	totalPages := (total + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"companies":   companies,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetCompany returns a specific company by ticker
func (h *UploadHandler) GetCompany(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ticker := strings.ToUpper(c.Param("ticker"))
	
	// Get company from service
	company, err := h.scraperService.GetCompanyByTicker(ctx, ticker)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Company with ticker %s not found", ticker)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch company: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"company": company})
}

// GetSystemHealth returns overall system health status
func (h *UploadHandler) GetSystemHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check overall system health
	err := h.scraperService.Health(ctx)
	healthy := err == nil
	
	// Get scraper health details
	scraperHealth := h.scraperService.GetScraperHealthStatus()
	
	response := gin.H{
		"healthy":         healthy,
		"timestamp":       time.Now(),
		"scraper_health":  scraperHealth,
	}
	
	if err != nil {
		response["error"] = err.Error()
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}
	
	c.JSON(http.StatusOK, response)
}

// GetScraperHealth returns detailed scraper health status
func (h *UploadHandler) GetScraperHealth(c *gin.Context) {
	healthStatus := h.scraperService.GetScraperHealthStatus()
	
	c.JSON(http.StatusOK, gin.H{
		"health_status": healthStatus,
		"timestamp":     time.Now(),
	})
}

// ResetScraperHealth resets the scraper health monitor
func (h *UploadHandler) ResetScraperHealth(c *gin.Context) {
	// Only allow admins to reset health monitor
	role, exists := c.Get("user_role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	h.scraperService.ResetScraperHealthMonitor()
	
	c.JSON(http.StatusOK, gin.H{
		"message":   "Scraper health monitor reset successfully",
		"timestamp": time.Now(),
	})
}