package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
)

// MockScraperService implements the scraper service interface for testing
type MockScraperService struct {
	jobs      map[uuid.UUID]*models.ScrapeJob
	companies map[string]*models.Company
}

func NewMockScraperService() *MockScraperService {
	return &MockScraperService{
		jobs:      make(map[uuid.UUID]*models.ScrapeJob),
		companies: make(map[string]*models.Company),
	}
}

func (m *MockScraperService) ScrapeTickersBatch(ctx context.Context, tickers []string, userID uuid.UUID, useOptimized bool) (*models.ScrapeJob, error) {
	job := &models.ScrapeJob{
		ID:               uuid.New(),
		Status:           string(models.ScrapeJobPending),
		TotalTickers:     len(tickers),
		ProcessedTickers: 0,
		FailedTickers:    0,
		StartedBy:        userID,
		StartedAt:        time.Now(),
	}
	m.jobs[job.ID] = job
	return job, nil
}

func (m *MockScraperService) GetUserJobs(ctx context.Context, userID uuid.UUID) ([]*models.ScrapeJob, error) {
	var jobs []*models.ScrapeJob
	for _, job := range m.jobs {
		if job.StartedBy == userID {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (m *MockScraperService) GetJob(ctx context.Context, jobID uuid.UUID) (*models.ScrapeJob, error) {
	job, exists := m.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found")
	}
	return job, nil
}

func (m *MockScraperService) GetCompanies(ctx context.Context, page, limit int, search, marketTier string) ([]models.Company, int, error) {
	var companies []models.Company
	for _, company := range m.companies {
		companies = append(companies, *company)
	}
	return companies, len(companies), nil
}

func (m *MockScraperService) GetCompanyByTicker(ctx context.Context, ticker string) (*models.Company, error) {
	company, exists := m.companies[ticker]
	if !exists {
		return nil, fmt.Errorf("company with ticker %s not found", ticker)
	}
	return company, nil
}

func TestUploadHandlerCSVUpload(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	mockService := NewMockScraperService()
	handler := NewUploadHandler(mockService)
	
	// Add a mock middleware to set user ID
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	
	router.POST("/upload/csv", handler.UploadCSV)

	// Create test CSV content
	csvContent := "ticker\nAAPL\nMSFT\nGOOGL\n"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	fileWriter, err := writer.CreateFormFile("csv_file", "test.csv")
	if err != nil {
		t.Fatal(err)
	}
	
	_, err = fileWriter.Write([]byte(csvContent))
	if err != nil {
		t.Fatal(err)
	}
	
	writer.WriteField("use_optimized", "false")
	writer.Close()

	// Make request
	req := httptest.NewRequest("POST", "/upload/csv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatal(err)
	}

	if response["total_tickers"] != float64(3) {
		t.Errorf("Expected 3 tickers, got %v", response["total_tickers"])
	}

	if !strings.Contains(response["message"].(string), "CSV upload successful") {
		t.Errorf("Unexpected response message: %s", response["message"])
	}
}

func TestUploadHandlerInvalidCSV(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	mockService := NewMockScraperService()
	handler := NewUploadHandler(mockService)
	
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	
	router.POST("/upload/csv", handler.UploadCSV)

	// Test with empty CSV
	csvContent := ""
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	fileWriter, _ := writer.CreateFormFile("csv_file", "empty.csv")
	fileWriter.Write([]byte(csvContent))
	writer.Close()

	req := httptest.NewRequest("POST", "/upload/csv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty CSV, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRoutesSetupWithoutPanic(t *testing.T) {
	// This test ensures that route setup returns an error instead of panicking
	// when there's an issue with service creation
	
	router := gin.New()
	
	// Pass nil values to trigger an error condition
	err := SetupRoutes(router, nil, nil)
	
	// Should return an error, not panic
	if err == nil {
		t.Error("Expected SetupRoutes to return an error with nil inputs")
	}
}