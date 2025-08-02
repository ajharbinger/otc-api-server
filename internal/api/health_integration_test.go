package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scraper"
)

func TestHealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Create mock service with health monitoring
	mockService := NewMockScraperService()
	handler := NewUploadHandler(mockService)
	
	// Add auth middleware mock
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Set("user_role", "admin")
		c.Next()
	})
	
	// Setup health routes
	router.GET("/health", handler.GetSystemHealth)
	router.GET("/health/scraper", handler.GetScraperHealth)
	router.POST("/health/scraper/reset", handler.ResetScraperHealth)

	t.Run("GetSystemHealth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			t.Fatal(err)
		}

		if _, exists := response["healthy"]; !exists {
			t.Error("Expected 'healthy' field in response")
		}

		if _, exists := response["timestamp"]; !exists {
			t.Error("Expected 'timestamp' field in response")
		}

		if _, exists := response["scraper_health"]; !exists {
			t.Error("Expected 'scraper_health' field in response")
		}
	})

	t.Run("GetScraperHealth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health/scraper", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			t.Fatal(err)
		}

		if _, exists := response["health_status"]; !exists {
			t.Error("Expected 'health_status' field in response")
		}

		if _, exists := response["timestamp"]; !exists {
			t.Error("Expected 'timestamp' field in response")
		}
	})

	t.Run("ResetScraperHealth_Admin", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/health/scraper/reset", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			t.Fatal(err)
		}

		if response["message"] != "Scraper health monitor reset successfully" {
			t.Error("Expected success message")
		}
	})

	t.Run("ResetScraperHealth_NonAdmin", func(t *testing.T) {
		// Create new router with non-admin user
		nonAdminRouter := gin.New()
		nonAdminRouter.Use(func(c *gin.Context) {
			c.Set("user_id", uuid.New())
			c.Set("user_role", "user") // Non-admin
			c.Next()
		})
		nonAdminRouter.POST("/health/scraper/reset", handler.ResetScraperHealth)

		req := httptest.NewRequest("POST", "/health/scraper/reset", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		nonAdminRouter.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})
}

// Add health monitoring methods to MockScraperService
func (m *MockScraperService) GetScraperHealthStatus() scraper.HealthStatus {
	return scraper.HealthStatus{
		IsHealthy:           true,
		TotalRequests:       100,
		SuccessfulRequests:  95,
		FailedRequests:      5,
		SuccessRate:         0.95,
		ConsecutiveFailures: 0,
		LastSuccessTime:     &time.Time{},
		RecentFailures:      []scraper.FailureRecord{},
		HealthIssues:        []string{},
		RecommendedActions:  []string{},
	}
}

func (m *MockScraperService) IsScraperHealthy() bool {
	return true
}

func (m *MockScraperService) GetScraperFailureRate() float64 {
	return 0.05
}

func (m *MockScraperService) ResetScraperHealthMonitor() {
	// Mock implementation - no-op
}

func (m *MockScraperService) Health(ctx context.Context) error {
	return nil
}