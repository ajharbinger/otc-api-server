package scraper

import (
	"testing"
	"time"
)

func TestHealthMonitor_RecordSuccessAndFailure(t *testing.T) {
	monitor := NewHealthMonitor()
	
	// Initially healthy
	if !monitor.IsHealthy() {
		t.Error("Expected new monitor to be healthy")
	}
	
	// Record some successes
	monitor.RecordSuccess("AAPL")
	monitor.RecordSuccess("MSFT")
	monitor.RecordSuccess("GOOGL")
	
	status := monitor.GetHealthStatus()
	if status.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", status.TotalRequests)
	}
	if status.SuccessfulRequests != 3 {
		t.Errorf("Expected 3 successful requests, got %d", status.SuccessfulRequests)
	}
	if status.SuccessRate != 1.0 {
		t.Errorf("Expected 100%% success rate, got %.2f", status.SuccessRate)
	}
	
	// Record a failure
	monitor.RecordFailure("BADTICKER", "network error", "https://example.com")
	
	status = monitor.GetHealthStatus()
	if status.TotalRequests != 4 {
		t.Errorf("Expected 4 total requests, got %d", status.TotalRequests)
	}
	if status.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", status.FailedRequests)
	}
	if status.SuccessRate != 0.75 {
		t.Errorf("Expected 75%% success rate, got %.2f", status.SuccessRate)
	}
	if len(status.RecentFailures) != 1 {
		t.Errorf("Expected 1 recent failure, got %d", len(status.RecentFailures))
	}
}

func TestHealthMonitor_ConsecutiveFailures(t *testing.T) {
	monitor := NewHealthMonitor()
	
	// Record multiple consecutive failures
	for i := 0; i < 6; i++ {
		monitor.RecordFailure("TICKER", "error", "")
	}
	
	status := monitor.GetHealthStatus()
	if status.IsHealthy {
		t.Error("Expected monitor to be unhealthy after consecutive failures")
	}
	if status.ConsecutiveFailures != 6 {
		t.Errorf("Expected 6 consecutive failures, got %d", status.ConsecutiveFailures)
	}
	
	// Check health issues
	found := false
	for _, issue := range status.HealthIssues {
		if issue == "Multiple consecutive failures detected" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected consecutive failure health issue")
	}
	
	// Record a success - should reset consecutive failures
	monitor.RecordSuccess("TICKER")
	
	status = monitor.GetHealthStatus()
	if status.ConsecutiveFailures != 0 {
		t.Errorf("Expected consecutive failures to reset, got %d", status.ConsecutiveFailures)
	}
}

func TestHealthMonitor_HighFailureRate(t *testing.T) {
	monitor := NewHealthMonitor()
	
	// Record enough requests to trigger failure rate check
	for i := 0; i < 5; i++ {
		monitor.RecordSuccess("TICKER")
	}
	for i := 0; i < 10; i++ {
		monitor.RecordFailure("TICKER", "error", "")
	}
	
	status := monitor.GetHealthStatus()
	if status.IsHealthy {
		t.Error("Expected monitor to be unhealthy due to high failure rate")
	}
	
	// Check for high failure rate issue
	found := false
	for _, issue := range status.HealthIssues {
		if issue == "High failure rate detected (>20%)" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected high failure rate health issue")
	}
}

func TestHealthMonitor_FailurePatternAnalysis(t *testing.T) {
	monitor := NewHealthMonitor()
	
	// Record many timeout errors
	for i := 0; i < 10; i++ {
		monitor.RecordFailure("TICKER", "timeout error", "")
	}
	
	status := monitor.GetHealthStatus()
	
	// Check for timeout-specific recommendations
	foundIssue := false
	foundAction := false
	
	for _, issue := range status.HealthIssues {
		if issue == "Frequent timeout errors detected" {
			foundIssue = true
			break
		}
	}
	
	for _, action := range status.RecommendedActions {
		if action == "Consider increasing request timeouts or reducing concurrency" {
			foundAction = true
			break
		}
	}
	
	if !foundIssue {
		t.Error("Expected timeout error pattern to be detected")
	}
	if !foundAction {
		t.Error("Expected timeout-specific recommended action")
	}
}

func TestHealthMonitor_RecentFailuresLimit(t *testing.T) {
	monitor := NewHealthMonitor()
	
	// Record more failures than the limit
	for i := 0; i < 60; i++ {
		monitor.RecordFailure("TICKER", "error", "")
	}
	
	status := monitor.GetHealthStatus()
	if len(status.RecentFailures) > monitor.maxRecentFailures {
		t.Errorf("Expected recent failures to be limited to %d, got %d", 
			monitor.maxRecentFailures, len(status.RecentFailures))
	}
}

func TestHealthMonitor_Reset(t *testing.T) {
	monitor := NewHealthMonitor()
	
	// Record some data
	monitor.RecordSuccess("TICKER")
	monitor.RecordFailure("TICKER", "error", "")
	
	// Reset
	monitor.Reset()
	
	status := monitor.GetHealthStatus()
	if status.TotalRequests != 0 {
		t.Errorf("Expected total requests to be 0 after reset, got %d", status.TotalRequests)
	}
	if status.SuccessfulRequests != 0 {
		t.Errorf("Expected successful requests to be 0 after reset, got %d", status.SuccessfulRequests)
	}
	if status.FailedRequests != 0 {
		t.Errorf("Expected failed requests to be 0 after reset, got %d", status.FailedRequests)
	}
	if len(status.RecentFailures) != 0 {
		t.Errorf("Expected recent failures to be empty after reset, got %d", len(status.RecentFailures))
	}
}

func TestHealthMonitor_FailureRateCalculation(t *testing.T) {
	monitor := NewHealthMonitor()
	
	// Test with no requests
	if monitor.GetFailureRate() != 0.0 {
		t.Error("Expected 0% failure rate with no requests")
	}
	
	// Test with mixed results
	monitor.RecordSuccess("TICKER")
	monitor.RecordSuccess("TICKER")
	monitor.RecordFailure("TICKER", "error", "")
	monitor.RecordFailure("TICKER", "error", "")
	
	expectedRate := 0.5 // 50%
	actualRate := monitor.GetFailureRate()
	if actualRate != expectedRate {
		t.Errorf("Expected failure rate %.2f, got %.2f", expectedRate, actualRate)
	}
}

func TestCategorizeError(t *testing.T) {
	testCases := []struct {
		error    string
		expected string
	}{
		{"connection timeout", "timeout"},
		{"context deadline exceeded", "timeout"},
		{"rate limit exceeded", "rate_limit"},
		{"HTTP 429", "rate_limit"},
		{"unauthorized access", "authentication"},
		{"HTTP 401", "authentication"},
		{"HTTP 403", "authentication"},
		{"network unreachable", "network"},
		{"DNS resolution failed", "network"},
		{"connection refused", "network"},
		{"unknown error", "other"},
	}
	
	for _, tc := range testCases {
		result := categorizeError(tc.error)
		if result != tc.expected {
			t.Errorf("categorizeError(%q) = %q, expected %q", tc.error, result, tc.expected)
		}
	}
}