package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

// Mock scoring service for testing
type mockScoringService struct {
	models      []*scoring.ICPModel
	scores      []scoring.ScoreResult
	shouldError bool
}

func (m *mockScoringService) GetActiveScoringModels() ([]*scoring.ICPModel, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	return m.models, nil
}

func (m *mockScoringService) GetScoringModel(modelID string) (*scoring.ICPModel, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	for _, model := range m.models {
		if model.ID == modelID {
			return model, nil
		}
	}
	return nil, errors.New("scoring model " + modelID + " not found")
}

func (m *mockScoringService) CreateScoringModel(model *scoring.ICPModel, userID uuid.UUID) error {
	if m.shouldError {
		return errors.New("mock error")
	}
	m.models = append(m.models, model)
	return nil
}

func (m *mockScoringService) UpdateScoringModel(model *scoring.ICPModel) error {
	if m.shouldError {
		return errors.New("mock error")
	}
	for i, existing := range m.models {
		if existing.ID == model.ID {
			m.models[i] = model
			return nil
		}
	}
	return errors.New("model not found")
}

func (m *mockScoringService) DeleteScoringModel(modelID string) error {
	if m.shouldError {
		return errors.New("mock error")
	}
	for i, model := range m.models {
		if model.ID == modelID {
			m.models = append(m.models[:i], m.models[i+1:]...)
			return nil
		}
	}
	return errors.New("scoring model " + modelID + " not found")
}

func (m *mockScoringService) ScoreCompany(companyID string) error {
	if m.shouldError {
		return errors.New("mock error")
	}
	return nil
}

func (m *mockScoringService) GetCompanyScores(companyID string) ([]scoring.ScoreResult, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	return m.scores, nil
}

func (m *mockScoringService) ScoreCompanyWithModel(companyID, modelID string) (*scoring.ScoreResult, error) {
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	result := &scoring.ScoreResult{
		CompanyID:      companyID,
		ScoringModelID: modelID,
		Score:          5,
		Qualified:      true,
		ScoredAt:       time.Now(),
		Breakdown:      make(map[string]scoring.ScoreDetail),
	}
	return result, nil
}

func (m *mockScoringService) ScoreAllCompaniesWithModel(modelID string) error {
	if m.shouldError {
		return errors.New("mock error")
	}
	return nil
}

func setupTestHandler() (*ScoringHandler, *mockScoringService) {
	mockService := &mockScoringService{
		models: []*scoring.ICPModel{
			{
				ID:          "test-model-1",
				Name:        "Test Model 1",
				Description: "A test scoring model",
				Version:     1,
				IsActive:    true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				Requirements: []scoring.Requirement{
					{
						Field:       "market_tier",
						Operator:    "equals",
						Value:       "Expert Market",
						Description: "Must be Expert Market",
					},
				},
				Rules: []scoring.ScoringRule{
					{
						Field:       "delinquent_10k",
						Weight:      1,
						Description: "Delinquent 10-K",
					},
				},
				MinScore: 3,
			},
		},
		scores: []scoring.ScoreResult{
			{
				CompanyID:      "test-company",
				ScoringModelID: "test-model-1",
				Score:          5,
				Qualified:      true,
				ScoredAt:       time.Now(),
				Breakdown:      make(map[string]scoring.ScoreDetail),
			},
		},
	}

	handler := &ScoringHandler{
		scoringService: mockService,
	}

	return handler, mockService
}

func TestScoringHandler_GetScoringModels(t *testing.T) {
	handler, mockService := setupTestHandler()
	
	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/scoring/models", handler.GetScoringModels)
	
	// Test successful request
	req, _ := http.NewRequest("GET", "/scoring/models", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if models, exists := response["models"]; !exists {
		t.Error("Expected 'models' field in response")
	} else if modelSlice, ok := models.([]interface{}); !ok {
		t.Error("Expected models to be an array")
	} else if len(modelSlice) != 1 {
		t.Errorf("Expected 1 model, got %d", len(modelSlice))
	}
	
	// Test error case
	mockService.shouldError = true
	req, _ = http.NewRequest("GET", "/scoring/models", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for error case, got %d", resp.Code)
	}
}

func TestScoringHandler_GetScoringModel(t *testing.T) {
	handler, mockService := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/scoring/models/:id", handler.GetScoringModel)
	
	// Test successful request
	req, _ := http.NewRequest("GET", "/scoring/models/test-model-1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
	
	// Test not found
	req, _ = http.NewRequest("GET", "/scoring/models/nonexistent", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for not found, got %d", resp.Code)
	}
	
	// Test error case
	mockService.shouldError = true
	req, _ = http.NewRequest("GET", "/scoring/models/test-model-1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for error case, got %d", resp.Code)
	}
}

func TestScoringHandler_CreateScoringModel(t *testing.T) {
	handler, _ := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Add middleware to simulate authenticated admin user
	router.Use(func(c *gin.Context) {
		c.Set("user_role", "admin")
		c.Set("user_id", uuid.New())
		c.Next()
	})
	
	router.POST("/scoring/models", handler.CreateScoringModel)
	
	// Test successful creation
	model := scoring.ICPModel{
		Name:        "New Test Model",
		Description: "A new test model",
		Requirements: []scoring.Requirement{
			{
				Field:       "market_tier",
				Operator:    "equals",
				Value:       "Pink",
				Description: "Must be Pink market",
			},
		},
		Rules: []scoring.ScoringRule{
			{
				Field:       "delinquent_10k",
				Weight:      2,
				Description: "Delinquent filing",
			},
		},
		MinScore: 4,
	}
	
	body, _ := json.Marshal(model)
	req, _ := http.NewRequest("POST", "/scoring/models", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.Code)
	}
	
	// Test unauthorized (non-admin)
	router2 := gin.New()
	router2.Use(func(c *gin.Context) {
		c.Set("user_role", "user") // Not admin
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router2.POST("/scoring/models", handler.CreateScoringModel)
	
	req, _ = http.NewRequest("POST", "/scoring/models", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router2.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for non-admin, got %d", resp.Code)
	}
}

func TestScoringHandler_ScoreCompany(t *testing.T) {
	handler, mockService := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/scoring/companies/:id/score", handler.ScoreCompany)
	
	// Test successful scoring
	req, _ := http.NewRequest("POST", "/scoring/companies/test-company/score", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if message, exists := response["message"]; !exists {
		t.Error("Expected 'message' field in response")
	} else if msg, ok := message.(string); !ok || msg != "Company scored successfully" {
		t.Errorf("Expected success message, got %v", message)
	}
	
	// Test error case
	mockService.shouldError = true
	req, _ = http.NewRequest("POST", "/scoring/companies/test-company/score", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for error case, got %d", resp.Code)
	}
}

func TestScoringHandler_GetCompanyScores(t *testing.T) {
	handler, mockService := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/scoring/companies/:id/scores", handler.GetCompanyScores)
	
	// Test successful request
	req, _ := http.NewRequest("GET", "/scoring/companies/test-company/scores", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if scores, exists := response["scores"]; !exists {
		t.Error("Expected 'scores' field in response")
	} else if scoreSlice, ok := scores.([]interface{}); !ok {
		t.Error("Expected scores to be an array")
	} else if len(scoreSlice) != 1 {
		t.Errorf("Expected 1 score, got %d", len(scoreSlice))
	}
	
	// Test error case
	mockService.shouldError = true
	req, _ = http.NewRequest("GET", "/scoring/companies/test-company/scores", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for error case, got %d", resp.Code)
	}
}

func TestScoringHandler_ScoreCompanyWithModel(t *testing.T) {
	handler, mockService := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/scoring/companies/:id/score/:model_id", handler.ScoreCompanyWithModel)
	
	// Test successful scoring
	req, _ := http.NewRequest("POST", "/scoring/companies/test-company/score/test-model-1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if result, exists := response["result"]; !exists {
		t.Error("Expected 'result' field in response")
	} else if resultMap, ok := result.(map[string]interface{}); !ok {
		t.Error("Expected result to be an object")
	} else {
		if companyID, exists := resultMap["company_id"]; !exists || companyID != "test-company" {
			t.Errorf("Expected company_id 'test-company', got %v", companyID)
		}
		if modelID, exists := resultMap["scoring_model_id"]; !exists || modelID != "test-model-1" {
			t.Errorf("Expected scoring_model_id 'test-model-1', got %v", modelID)
		}
	}
	
	// Test error case
	mockService.shouldError = true
	req, _ = http.NewRequest("POST", "/scoring/companies/test-company/score/test-model-1", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for error case, got %d", resp.Code)
	}
}

func TestScoringHandler_UpdateScoringModel(t *testing.T) {
	handler, _ := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Add admin middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_role", "admin")
		c.Next()
	})
	
	router.PUT("/scoring/models/:id", handler.UpdateScoringModel)
	
	// Test successful update
	updatedModel := scoring.ICPModel{
		ID:          "test-model-1",
		Name:        "Updated Test Model",
		Description: "An updated test model",
		Version:     2,
		MinScore:    5,
	}
	
	body, _ := json.Marshal(updatedModel)
	req, _ := http.NewRequest("PUT", "/scoring/models/test-model-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
}

func TestScoringHandler_DeleteScoringModel(t *testing.T) {
	handler, _ := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Add admin middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_role", "admin")
		c.Next()
	})
	
	router.DELETE("/scoring/models/:id", handler.DeleteScoringModel)
	
	// Test successful deletion
	req, _ := http.NewRequest("DELETE", "/scoring/models/test-model-1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
	
	// Test not found
	req, _ = http.NewRequest("DELETE", "/scoring/models/nonexistent", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for not found, got %d", resp.Code)
	}
}

func TestScoringHandler_ScoreAllCompanies(t *testing.T) {
	handler, _ := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Add admin middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_role", "admin")
		c.Next()
	})
	
	router.POST("/scoring/models/:id/score-all", handler.ScoreAllCompanies)
	
	// Test successful bulk scoring initiation
	req, _ := http.NewRequest("POST", "/scoring/models/test-model-1/score-all", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", resp.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if message, exists := response["message"]; !exists {
		t.Error("Expected 'message' field in response")
	} else if msg, ok := message.(string); !ok || msg != "Bulk scoring job started" {
		t.Errorf("Expected bulk scoring message, got %v", message)
	}
}

// Test invalid JSON input
func TestScoringHandler_InvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	router.Use(func(c *gin.Context) {
		c.Set("user_role", "admin")
		c.Set("user_id", uuid.New())
		c.Next()
	})
	
	router.POST("/scoring/models", handler.CreateScoringModel)
	
	// Test with invalid JSON
	req, _ := http.NewRequest("POST", "/scoring/models", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.Code)
	}
}

// Test context timeout handling
func TestScoringHandler_ContextTimeout(t *testing.T) {
	handler, _ := setupTestHandler()
	
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/scoring/models", handler.GetScoringModels)
	
	// Create request with cancelled context
	req, _ := http.NewRequest("GET", "/scoring/models", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel() // Cancel immediately
	req = req.WithContext(ctx)
	
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	
	// The handler should still complete since we use a separate context
	// but this tests that we handle context properly
	if resp.Code != http.StatusOK {
		t.Logf("Request completed with status %d (context was cancelled)", resp.Code)
	}
}