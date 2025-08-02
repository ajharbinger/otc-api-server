package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/services"
)

// LeadsHandler handles lead filtering and export operations
type LeadsHandler struct {
	leadExportService *services.LeadExportService
}

// NewLeadsHandler creates a new leads handler
func NewLeadsHandler(db *sql.DB, scoringService services.ScoringService) *LeadsHandler {
	return &LeadsHandler{
		leadExportService: services.NewLeadExportService(db, scoringService),
	}
}

// GetQualifiedLeads returns qualified leads based on filter criteria
func (h *LeadsHandler) GetQualifiedLeads(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Parse filter from request
	filter, err := h.parseFilterFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filter parameters: " + err.Error()})
		return
	}

	leads, err := h.leadExportService.GetQualifiedLeads(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get qualified leads: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"leads":     leads,
		"count":     len(leads),
		"filter":    filter,
		"timestamp": time.Now(),
	})
}

// ExportQualifiedLeads exports qualified leads in the specified format
func (h *LeadsHandler) ExportQualifiedLeads(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Parse filter from request body or query
	var filter services.LeadFilter
	if err := c.ShouldBindJSON(&filter); err != nil {
		// Fallback to query parameters
		var parseErr error
		filter, parseErr = h.parseFilterFromQuery(c)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filter parameters: " + parseErr.Error()})
			return
		}
	}

	// Parse export options
	options := services.LeadExportOptions{
		Format:                services.FormatJSON, // Default to JSON
		IncludeScoreBreakdown: false,
		IncludeMetadata:       true,
	}

	if format := c.Query("format"); format != "" {
		switch strings.ToLower(format) {
		case "csv":
			options.Format = services.FormatCSV
		case "json":
			options.Format = services.FormatJSON
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Supported formats: json, csv"})
			return
		}
	}

	if includeBreakdown := c.Query("include_breakdown"); includeBreakdown == "true" {
		options.IncludeScoreBreakdown = true
	}

	if includeMetadata := c.Query("include_metadata"); includeMetadata == "false" {
		options.IncludeMetadata = false
	}

	// Export leads
	data, err := h.leadExportService.ExportQualifiedLeads(filter, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export leads: " + err.Error()})
		return
	}

	// Set appropriate headers
	filename := "qualified_leads_" + time.Now().Format("2006-01-02_15-04-05")
	
	switch options.Format {
	case services.FormatCSV:
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
	case services.FormatJSON:
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", `attachment; filename="`+filename+`.json"`)
	}

	c.Data(http.StatusOK, c.GetHeader("Content-Type"), data)
}

// GetLeadStats returns statistics about qualified leads
func (h *LeadsHandler) GetLeadStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse optional filter
	filter, _ := h.parseFilterFromQuery(c)

	// Get leads to calculate stats
	leads, err := h.leadExportService.GetQualifiedLeads(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get lead statistics: " + err.Error()})
		return
	}

	// Calculate statistics
	stats := h.calculateLeadStats(leads)

	c.JSON(http.StatusOK, gin.H{
		"stats":     stats,
		"filter":    filter,
		"timestamp": time.Now(),
	})
}

// GetLeadByTicker returns detailed information about a specific company lead
func (h *LeadsHandler) GetLeadByTicker(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ticker := strings.ToUpper(c.Param("ticker"))
	
	// Create filter for specific ticker
	filter := services.LeadFilter{
		Limit: &[]int{1}[0], // Limit to 1 result
	}

	// We need to add a ticker filter to the service - for now use a broader search
	leads, err := h.leadExportService.GetQualifiedLeads(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get lead details: " + err.Error()})
		return
	}

	// Find matching ticker
	for _, lead := range leads {
		if lead.Ticker == ticker {
			c.JSON(http.StatusOK, gin.H{
				"lead":      lead,
				"timestamp": time.Now(),
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Lead not found for ticker: " + ticker})
}

// parseFilterFromQuery parses filter criteria from query parameters
func (h *LeadsHandler) parseFilterFromQuery(c *gin.Context) (services.LeadFilter, error) {
	filter := services.LeadFilter{}

	// Parse model IDs
	if modelIDs := c.Query("model_ids"); modelIDs != "" {
		filter.ModelIDs = strings.Split(modelIDs, ",")
	}

	// Parse score range
	if minScore := c.Query("min_score"); minScore != "" {
		if parsed, err := strconv.Atoi(minScore); err == nil {
			filter.MinScore = &parsed
		}
	}

	if maxScore := c.Query("max_score"); maxScore != "" {
		if parsed, err := strconv.Atoi(maxScore); err == nil {
			filter.MaxScore = &parsed
		}
	}

	// Parse market tiers
	if tiers := c.Query("market_tiers"); tiers != "" {
		filter.MarketTiers = strings.Split(tiers, ",")
	}

	// Parse quote statuses
	if statuses := c.Query("quote_statuses"); statuses != "" {
		filter.QuoteStatuses = strings.Split(statuses, ",")
	}

	// Parse date ranges
	if scoredAfter := c.Query("scored_after"); scoredAfter != "" {
		if parsed, err := time.Parse("2006-01-02", scoredAfter); err == nil {
			filter.ScoredAfter = &parsed
		}
	}

	if scoredBefore := c.Query("scored_before"); scoredBefore != "" {
		if parsed, err := time.Parse("2006-01-02", scoredBefore); err == nil {
			filter.ScoredBefore = &parsed
		}
	}

	// Parse trading volume range
	if minVolume := c.Query("trading_volume_min"); minVolume != "" {
		if parsed, err := strconv.ParseInt(minVolume, 10, 64); err == nil {
			filter.TradingVolumeMin = &parsed
		}
	}

	if maxVolume := c.Query("trading_volume_max"); maxVolume != "" {
		if parsed, err := strconv.ParseInt(maxVolume, 10, 64); err == nil {
			filter.TradingVolumeMax = &parsed
		}
	}

	// Parse boolean filters
	if hasWebsite := c.Query("has_website"); hasWebsite != "" {
		if parsed, err := strconv.ParseBool(hasWebsite); err == nil {
			filter.HasWebsite = &parsed
		}
	}

	if hasTransferAgent := c.Query("has_transfer_agent"); hasTransferAgent != "" {
		if parsed, err := strconv.ParseBool(hasTransferAgent); err == nil {
			filter.HasTransferAgent = &parsed
		}
	}

	if hasAuditor := c.Query("has_auditor"); hasAuditor != "" {
		if parsed, err := strconv.ParseBool(hasAuditor); err == nil {
			filter.HasAuditor = &parsed
		}
	}

	// Parse other options
	if includeRequiredOnly := c.Query("include_required_only"); includeRequiredOnly == "true" {
		filter.IncludeRequiredOnly = true
	}

	if limit := c.Query("limit"); limit != "" {
		if parsed, err := strconv.Atoi(limit); err == nil {
			filter.Limit = &parsed
		}
	}

	return filter, nil
}

// calculateLeadStats calculates statistics from a slice of leads
func (h *LeadsHandler) calculateLeadStats(leads []services.QualifiedLead) map[string]interface{} {
	stats := map[string]interface{}{
		"total_leads": len(leads),
	}

	if len(leads) == 0 {
		return stats
	}

	// Market tier distribution
	tierCounts := make(map[string]int)
	modelCounts := make(map[string]int)
	scoreSum := 0
	minScore := leads[0].Score
	maxScore := leads[0].Score

	for _, lead := range leads {
		tierCounts[lead.MarketTier]++
		modelCounts[lead.ModelName]++
		scoreSum += lead.Score

		if lead.Score < minScore {
			minScore = lead.Score
		}
		if lead.Score > maxScore {
			maxScore = lead.Score
		}
	}

	stats["market_tier_distribution"] = tierCounts
	stats["model_distribution"] = modelCounts
	stats["average_score"] = float64(scoreSum) / float64(len(leads))
	stats["min_score"] = minScore
	stats["max_score"] = maxScore

	// Risk indicator analysis
	riskCounts := make(map[string]int)
	for _, lead := range leads {
		for _, risk := range lead.RiskIndicators {
			riskCounts[risk]++
		}
	}
	stats["common_risk_indicators"] = riskCounts

	// Service recommendations
	serviceCounts := make(map[string]int)
	for _, lead := range leads {
		for _, service := range lead.RecommendedServices {
			serviceCounts[service]++
		}
	}
	stats["recommended_services"] = serviceCounts

	return stats
}