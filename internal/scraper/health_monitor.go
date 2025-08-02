package scraper

import (
	"strings"
	"sync"
	"time"
)

// HealthMonitor tracks scraper performance and failure rates
type HealthMonitor struct {
	mu                    sync.RWMutex
	totalRequests         int64
	successfulRequests    int64
	failedRequests        int64
	consecutiveFailures   int64
	lastFailureTime       time.Time
	lastSuccessTime       time.Time
	recentFailures        []FailureRecord
	maxRecentFailures     int
	failureThreshold      float64 // Percentage threshold for high failure rate
	consecutiveThreshold  int64   // Max consecutive failures before alerting
}

// FailureRecord represents a single failure event
type FailureRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Ticker    string    `json:"ticker"`
	Error     string    `json:"error"`
	URL       string    `json:"url,omitempty"`
}

// HealthStatus represents the current health status of the scraper
type HealthStatus struct {
	IsHealthy               bool              `json:"is_healthy"`
	TotalRequests          int64             `json:"total_requests"`
	SuccessfulRequests     int64             `json:"successful_requests"`
	FailedRequests         int64             `json:"failed_requests"`
	SuccessRate            float64           `json:"success_rate"`
	ConsecutiveFailures    int64             `json:"consecutive_failures"`
	LastFailureTime        *time.Time        `json:"last_failure_time,omitempty"`
	LastSuccessTime        *time.Time        `json:"last_success_time,omitempty"`
	RecentFailures         []FailureRecord   `json:"recent_failures"`
	HealthIssues           []string          `json:"health_issues"`
	RecommendedActions     []string          `json:"recommended_actions"`
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		maxRecentFailures:    50, // Keep last 50 failures
		failureThreshold:     0.2, // Alert if failure rate > 20%
		consecutiveThreshold: 5,   // Alert after 5 consecutive failures
		recentFailures:       make([]FailureRecord, 0, 50),
	}
}

// RecordSuccess records a successful scraping operation
func (h *HealthMonitor) RecordSuccess(ticker string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.totalRequests++
	h.successfulRequests++
	h.consecutiveFailures = 0
	h.lastSuccessTime = time.Now()
}

// RecordFailure records a failed scraping operation
func (h *HealthMonitor) RecordFailure(ticker, errorMsg, url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.totalRequests++
	h.failedRequests++
	h.consecutiveFailures++
	h.lastFailureTime = time.Now()
	
	// Add to recent failures (keep only the most recent)
	failure := FailureRecord{
		Timestamp: time.Now(),
		Ticker:    ticker,
		Error:     errorMsg,
		URL:       url,
	}
	
	h.recentFailures = append(h.recentFailures, failure)
	if len(h.recentFailures) > h.maxRecentFailures {
		h.recentFailures = h.recentFailures[1:]
	}
}

// GetHealthStatus returns the current health status
func (h *HealthMonitor) GetHealthStatus() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	status := HealthStatus{
		TotalRequests:       h.totalRequests,
		SuccessfulRequests:  h.successfulRequests,
		FailedRequests:      h.failedRequests,
		ConsecutiveFailures: h.consecutiveFailures,
		RecentFailures:      make([]FailureRecord, len(h.recentFailures)),
		HealthIssues:        []string{},
		RecommendedActions:  []string{},
	}
	
	// Copy recent failures
	copy(status.RecentFailures, h.recentFailures)
	
	// Calculate success rate
	if h.totalRequests > 0 {
		status.SuccessRate = float64(h.successfulRequests) / float64(h.totalRequests)
	} else {
		status.SuccessRate = 1.0
	}
	
	// Set timestamps if available
	if !h.lastFailureTime.IsZero() {
		status.LastFailureTime = &h.lastFailureTime
	}
	if !h.lastSuccessTime.IsZero() {
		status.LastSuccessTime = &h.lastSuccessTime
	}
	
	// Determine health status and issues
	status.IsHealthy = true
	
	// Check failure rate
	if h.totalRequests >= 10 && status.SuccessRate < (1.0-h.failureThreshold) {
		status.IsHealthy = false
		status.HealthIssues = append(status.HealthIssues, 
			"High failure rate detected (>20%)")
		status.RecommendedActions = append(status.RecommendedActions,
			"Check OxyLabs connectivity and credentials")
	}
	
	// Check consecutive failures
	if h.consecutiveFailures >= h.consecutiveThreshold {
		status.IsHealthy = false
		status.HealthIssues = append(status.HealthIssues,
			"Multiple consecutive failures detected")
		status.RecommendedActions = append(status.RecommendedActions,
			"Verify OTC Markets website accessibility and rate limits")
	}
	
	// Check if no successful requests in last hour
	if !h.lastSuccessTime.IsZero() && time.Since(h.lastSuccessTime) > time.Hour {
		status.IsHealthy = false
		status.HealthIssues = append(status.HealthIssues,
			"No successful requests in the last hour")
		status.RecommendedActions = append(status.RecommendedActions,
			"Check system connectivity and OxyLabs service status")
	}
	
	// Check for specific error patterns in recent failures
	h.analyzeFailurePatterns(&status)
	
	return status
}

// analyzeFailurePatterns looks for patterns in recent failures
func (h *HealthMonitor) analyzeFailurePatterns(status *HealthStatus) {
	if len(h.recentFailures) < 3 {
		return
	}
	
	// Count error types in recent failures
	errorCounts := make(map[string]int)
	for _, failure := range h.recentFailures {
		// Categorize errors
		errorType := categorizeError(failure.Error)
		errorCounts[errorType]++
	}
	
	totalRecent := len(h.recentFailures)
	for errorType, count := range errorCounts {
		if float64(count)/float64(totalRecent) > 0.5 { // >50% of recent failures
			switch errorType {
			case "timeout":
				status.HealthIssues = append(status.HealthIssues,
					"Frequent timeout errors detected")
				status.RecommendedActions = append(status.RecommendedActions,
					"Consider increasing request timeouts or reducing concurrency")
			case "rate_limit":
				status.HealthIssues = append(status.HealthIssues,
					"Rate limiting detected")
				status.RecommendedActions = append(status.RecommendedActions,
					"Reduce scraping frequency and implement exponential backoff")
			case "authentication":
				status.HealthIssues = append(status.HealthIssues,
					"Authentication errors detected")
				status.RecommendedActions = append(status.RecommendedActions,
					"Verify OxyLabs credentials and account status")
			case "network":
				status.HealthIssues = append(status.HealthIssues,
					"Network connectivity issues detected")
				status.RecommendedActions = append(status.RecommendedActions,
					"Check network connectivity and DNS resolution")
			}
		}
	}
}

// categorizeError categorizes an error message into a type
func categorizeError(errorMsg string) string {
	errorMsg = strings.ToLower(errorMsg)
	
	if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "deadline") {
		return "timeout"
	}
	if strings.Contains(errorMsg, "rate limit") || strings.Contains(errorMsg, "429") {
		return "rate_limit"
	}
	if strings.Contains(errorMsg, "unauthorized") || strings.Contains(errorMsg, "401") || strings.Contains(errorMsg, "403") {
		return "authentication"
	}
	if strings.Contains(errorMsg, "network") || strings.Contains(errorMsg, "connection") || strings.Contains(errorMsg, "dns") {
		return "network"
	}
	
	return "other"
}

// Reset clears all health monitoring data
func (h *HealthMonitor) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.totalRequests = 0
	h.successfulRequests = 0
	h.failedRequests = 0
	h.consecutiveFailures = 0
	h.lastFailureTime = time.Time{}
	h.lastSuccessTime = time.Time{}
	h.recentFailures = h.recentFailures[:0]
}

// IsHealthy returns true if the scraper is operating within healthy parameters
func (h *HealthMonitor) IsHealthy() bool {
	return h.GetHealthStatus().IsHealthy
}

// GetFailureRate returns the current failure rate as a percentage
func (h *HealthMonitor) GetFailureRate() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if h.totalRequests == 0 {
		return 0.0
	}
	
	return float64(h.failedRequests) / float64(h.totalRequests)
}