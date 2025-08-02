package api

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/auth"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/services"
)

// ScoringHandler handles ICP scoring operations
type ScoringHandler struct {
	scoringService services.ScoringService
}

// NewScoringHandler creates a new scoring handler (legacy)
func NewScoringHandler(db *sql.DB) *ScoringHandler {
	return &ScoringHandler{
		scoringService: services.NewScoringServiceLegacy(db),
	}
}

// NewScoringHandlerWithService creates a new scoring handler with service injection
func NewScoringHandlerWithService(scoringService services.ScoringService) *ScoringHandler {
	return &ScoringHandler{
		scoringService: scoringService,
	}
}

// GetScoringModels returns all active ICP scoring models
func (h *ScoringHandler) GetScoringModels(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := h.scoringService.GetActiveScoringModels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scoring models: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"models":    models,
		"timestamp": time.Now(),
	})
}

// GetScoringModel returns a specific ICP scoring model
func (h *ScoringHandler) GetScoringModel(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	modelID := c.Param("id")
	
	model, err := h.scoringService.GetScoringModel(modelID)
	if err != nil {
		if err.Error() == "scoring model "+modelID+" not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scoring model not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scoring model: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"model":     model,
		"timestamp": time.Now(),
	})
}

// CreateScoringModel creates a new ICP scoring model (Admin only)
func (h *ScoringHandler) CreateScoringModel(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("user_role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Get user ID
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

	var model scoring.ICPModel
	if err := c.ShouldBindJSON(&model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model format: " + err.Error()})
		return
	}

	// Set defaults
	model.Version = 1
	model.IsActive = true

	if err := h.scoringService.CreateScoringModel(&model, userUUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create scoring model: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Scoring model created successfully",
		"model":     model,
		"timestamp": time.Now(),
	})
}

// UpdateScoringModel updates an existing ICP scoring model (Admin only)
func (h *ScoringHandler) UpdateScoringModel(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("user_role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	modelID := c.Param("id")

	var model scoring.ICPModel
	if err := c.ShouldBindJSON(&model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model format: " + err.Error()})
		return
	}

	// Ensure ID matches URL parameter
	model.ID = modelID

	if err := h.scoringService.UpdateScoringModel(&model); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update scoring model: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Scoring model updated successfully",
		"model":     model,
		"timestamp": time.Now(),
	})
}

// DeleteScoringModel soft deletes an ICP scoring model (Admin only)
func (h *ScoringHandler) DeleteScoringModel(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("user_role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	modelID := c.Param("id")

	if err := h.scoringService.DeleteScoringModel(modelID); err != nil {
		if err.Error() == "scoring model "+modelID+" not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scoring model not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete scoring model: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Scoring model deleted successfully",
		"timestamp": time.Now(),
	})
}

// ScoreCompany scores a company against all active ICP models
func (h *ScoringHandler) ScoreCompany(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	companyID := c.Param("id")

	if err := h.scoringService.ScoreCompany(companyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to score company: " + err.Error()})
		return
	}

	// Get the updated scores
	scores, err := h.scoringService.GetCompanyScores(companyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get company scores: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Company scored successfully",
		"company_id": companyID,
		"scores":    scores,
		"timestamp": time.Now(),
	})
}

// GetCompanyScores returns all scores for a company
func (h *ScoringHandler) GetCompanyScores(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	companyID := c.Param("id")

	scores, err := h.scoringService.GetCompanyScores(companyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get company scores: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"company_id": companyID,
		"scores":     scores,
		"timestamp":  time.Now(),
	})
}

// ScoreCompanyWithModel scores a company against a specific ICP model
func (h *ScoringHandler) ScoreCompanyWithModel(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	companyID := c.Param("id")
	modelID := c.Param("model_id")

	result, err := h.scoringService.ScoreCompanyWithModel(companyID, modelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to score company with model: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Company scored successfully with model",
		"company_id": companyID,
		"model_id":   modelID,
		"result":     result,
		"timestamp":  time.Now(),
	})
}

// ScoreAllCompanies scores all companies against a specific ICP model (Admin only)
func (h *ScoringHandler) ScoreAllCompanies(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("user_role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	modelID := c.Param("id")

	// Start scoring in background
	go func() {
		if err := h.scoringService.ScoreAllCompaniesWithModel(modelID); err != nil {
			// Log error - in production, this would go to proper logging system
			// log.Printf("Error scoring all companies with model %s: %v", modelID, err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":   "Bulk scoring job started",
		"model_id":  modelID,
		"timestamp": time.Now(),
	})
}