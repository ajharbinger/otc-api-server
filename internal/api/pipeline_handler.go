package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/services"
)

// PipelineHandler handles scoring pipeline management operations
type PipelineHandler struct {
	pipeline *services.ScoringPipeline
}

// NewPipelineHandler creates a new pipeline handler
func NewPipelineHandler(db *sql.DB) *PipelineHandler {
	return &PipelineHandler{
		pipeline: services.NewScoringPipeline(db),
	}
}

// GetPipelineStatus returns the current status of the scoring pipeline
func (h *PipelineHandler) GetPipelineStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := h.pipeline.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pipeline status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pipeline_status": status,
		"timestamp":       time.Now(),
	})
}

// StartPipeline starts the automated scoring pipeline (Admin only)
func (h *PipelineHandler) StartPipeline(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("user_role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Parse configuration from request body or use defaults
	var config services.PipelineConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		// Use default config if parsing fails
		config = services.DefaultPipelineConfig()
	}

	// Validate config
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	if config.IntervalMinutes <= 0 {
		config.IntervalMinutes = 60
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}
	if config.RescoreOlderThanDays <= 0 {
		config.RescoreOlderThanDays = 7
	}

	if err := h.pipeline.Start(config); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Failed to start pipeline: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Scoring pipeline started successfully",
		"config":    config,
		"timestamp": time.Now(),
	})
}

// StopPipeline stops the automated scoring pipeline (Admin only)
func (h *PipelineHandler) StopPipeline(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("user_role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	if err := h.pipeline.Stop(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Failed to stop pipeline: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Scoring pipeline stopped successfully",
		"timestamp": time.Now(),
	})
}

// RunPipelineOnce executes a single scoring cycle manually (Admin only)
func (h *PipelineHandler) RunPipelineOnce(c *gin.Context) {
	// Check admin role
	role, exists := c.Get("user_role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	// Parse configuration from query parameters or use defaults
	config := services.DefaultPipelineConfig()

	if batchSize := c.Query("batch_size"); batchSize != "" {
		if parsed, err := strconv.Atoi(batchSize); err == nil && parsed > 0 {
			config.BatchSize = parsed
		}
	}

	if maxConcurrent := c.Query("max_concurrent"); maxConcurrent != "" {
		if parsed, err := strconv.Atoi(maxConcurrent); err == nil && parsed > 0 {
			config.MaxConcurrent = parsed
		}
	}

	if processNewOnly := c.Query("process_new_only"); processNewOnly == "true" {
		config.ProcessNewOnly = true
	}

	if rescoreOlderThan := c.Query("rescore_older_than_days"); rescoreOlderThan != "" {
		if parsed, err := strconv.Atoi(rescoreOlderThan); err == nil && parsed > 0 {
			config.RescoreOlderThanDays = parsed
		}
	}

	// Execute one-time scoring
	stats, err := h.pipeline.RunOnce(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to run scoring cycle: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Scoring cycle completed successfully",
		"config":    config,
		"stats":     stats,
		"timestamp": time.Now(),
	})
}

// GetPipelineConfig returns the default pipeline configuration
func (h *PipelineHandler) GetPipelineConfig(c *gin.Context) {
	config := services.DefaultPipelineConfig()

	c.JSON(http.StatusOK, gin.H{
		"default_config": config,
		"description": map[string]string{
			"batch_size":             "Number of companies to process in each batch",
			"interval_minutes":       "How often to run scoring cycles (in minutes)",
			"max_concurrent":         "Maximum number of concurrent scoring operations",
			"process_new_only":       "Only process companies that have never been scored",
			"rescore_older_than_days": "Rescore companies that were scored more than X days ago",
		},
		"timestamp": time.Now(),
	})
}