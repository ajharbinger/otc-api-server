package scoring

import (
	"encoding/json"
	"testing"
	"time"
)

func TestScoringEngine_ScoreCompany(t *testing.T) {
	engine := NewScoringEngine()
	
	// Test data representing a company in Expert Market
	companyData := map[string]interface{}{
		"ticker":           "ABCD",
		"company_name":     "Test Company Inc",
		"market_tier":      "Expert Market",
		"quote_status":     "Ineligible for solicited quotes",
		"trading_volume":   100,
		"website":          "https://testcompany.com",
		"description":      "A test company for verification",
		"transfer_agent":   "Computershare Trust Company",
		"auditor":          "Test CPA Firm",
		"last_10k_date":    time.Now().AddDate(0, -18, 0), // 18 months ago
		"last_10q_date":    time.Now().AddDate(0, -8, 0),  // 8 months ago
		"last_filing_date": time.Now().AddDate(0, -3, 0),  // 3 months ago
		"profile_verified": false,
	}
	
	// Get Double Black Diamond ICP model
	model := engine.GetDoubleBlackDiamondICP()
	
	// Score the company
	result, err := engine.ScoreCompany(companyData, model)
	if err != nil {
		t.Fatalf("Failed to score company: %v", err)
	}
	
	// Verify basic result structure
	if result.ScoringModelID != model.ID {
		t.Errorf("Expected scoring model ID %s, got %s", model.ID, result.ScoringModelID)
	}
	
	if !result.RequirementsMet {
		t.Error("Expected requirements to be met for Expert Market company")
	}
	
	if result.Score <= 0 {
		t.Error("Expected positive score for delinquent company")
	}
	
	// Check specific scoring components
	if detail, exists := result.Breakdown["delinquent_10k"]; !exists || !detail.Triggered {
		t.Error("Expected delinquent_10k to be triggered for 18-month-old filing")
	}
	
	if detail, exists := result.Breakdown["delinquent_10q"]; !exists || !detail.Triggered {
		t.Error("Expected delinquent_10q to be triggered for 8-month-old filing")
	}
	
	if detail, exists := result.Breakdown["no_verified_profile"]; !exists || !detail.Triggered {
		t.Error("Expected no_verified_profile to be triggered")
	}
	
	// Quality signals should reduce score
	if detail, exists := result.Breakdown["active_transfer_agent"]; exists && detail.Triggered {
		if detail.Points >= 0 {
			t.Error("Expected active_transfer_agent to have negative points (quality signal)")
		}
	}
}

func TestScoringEngine_LoadICPModelFromJSON(t *testing.T) {
	engine := NewScoringEngine()
	
	// Test JSON data for an ICP model
	rulesJSON := []byte(`{
		"must_have": [
			{
				"field": "market_tier",
				"operator": "equals",
				"value": "Expert Market",
				"description": "Must be in Expert Market"
			}
		],
		"must_not": [
			{
				"field": "last_filing_date",
				"operator": "less_than",
				"value": "2020-01-01",
				"description": "Must not have very old filings"
			}
		],
		"scoring_rules": [
			{
				"field": "delinquent_10k",
				"operator": "is_true",
				"value": true,
				"weight": 2,
				"description": "Delinquent 10-K filing"
			},
			{
				"field": "active_transfer_agent",
				"operator": "is_true",
				"value": true,
				"weight": -1,
				"description": "Has reputable transfer agent"
			}
		],
		"minimum_score": 3
	}`)
	
	model, err := engine.LoadICPModelFromJSON(
		"test-model",
		"Test Model",
		"A test scoring model",
		1,
		rulesJSON,
		true,
		time.Now(),
		time.Now(),
	)
	
	if err != nil {
		t.Fatalf("Failed to load ICP model from JSON: %v", err)
	}
	
	// Verify model properties
	if model.ID != "test-model" {
		t.Errorf("Expected ID 'test-model', got %s", model.ID)
	}
	
	if len(model.Requirements) != 1 {
		t.Errorf("Expected 1 requirement, got %d", len(model.Requirements))
	}
	
	if len(model.Exclusions) != 1 {
		t.Errorf("Expected 1 exclusion, got %d", len(model.Exclusions))
	}
	
	if len(model.Rules) != 2 {
		t.Errorf("Expected 2 scoring rules, got %d", len(model.Rules))
	}
	
	if model.MinScore != 3 {
		t.Errorf("Expected minimum score 3, got %d", model.MinScore)
	}
}

func TestScoringEngine_EvaluateConditions(t *testing.T) {
	engine := NewScoringEngine()
	
	testCases := []struct {
		name      string
		data      map[string]interface{}
		field     string
		operator  string
		value     interface{}
		expected  bool
	}{
		{
			name:     "String equals match",
			data:     map[string]interface{}{"market_tier": "Expert Market"},
			field:    "market_tier",
			operator: "equals",
			value:    "Expert Market",
			expected: true,
		},
		{
			name:     "String equals no match",
			data:     map[string]interface{}{"market_tier": "OTC Pink"},
			field:    "market_tier",
			operator: "equals",
			value:    "Expert Market",
			expected: false,
		},
		{
			name:     "Contains match",
			data:     map[string]interface{}{"quote_status": "Ineligible for solicited quotes"},
			field:    "quote_status",
			operator: "contains",
			value:    "Ineligible",
			expected: true,
		},
		{
			name:     "Greater than true",
			data:     map[string]interface{}{"trading_volume": 1000},
			field:    "trading_volume",
			operator: "greater_than",
			value:    500,
			expected: true,
		},
		{
			name:     "Greater than false",
			data:     map[string]interface{}{"trading_volume": 300},
			field:    "trading_volume",
			operator: "greater_than",
			value:    500,
			expected: false,
		},
		{
			name:     "Boolean is_true",
			data:     map[string]interface{}{"profile_verified": true},
			field:    "profile_verified",
			operator: "is_true",
			value:    true,
			expected: true,
		},
		{
			name:     "Boolean is_false",
			data:     map[string]interface{}{"profile_verified": false},
			field:    "profile_verified",
			operator: "is_false",
			value:    false,
			expected: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := engine.evaluateCondition(tc.data, tc.field, tc.operator, tc.value)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestScoringEngine_EvaluateDelinquency(t *testing.T) {
	engine := NewScoringEngine()
	
	testCases := []struct {
		name           string
		date           interface{}
		monthsThreshold int
		expected       bool
	}{
		{
			name:           "Recent date not delinquent",
			date:           time.Now().AddDate(0, -3, 0), // 3 months ago
			monthsThreshold: 6,
			expected:       false,
		},
		{
			name:           "Old date is delinquent",
			date:           time.Now().AddDate(0, -18, 0), // 18 months ago
			monthsThreshold: 15,
			expected:       true,
		},
		{
			name:           "Nil date is delinquent",
			date:           nil,
			monthsThreshold: 6,
			expected:       true,
		},
		{
			name:           "String date parsing",
			date:           "2020-01-01",
			monthsThreshold: 12,
			expected:       true, // Should be delinquent (very old)
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]interface{}{"test_date": tc.date}
			result := engine.evaluateDelinquency(data, "test_date", tc.monthsThreshold)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestScoringEngine_EvaluateDescriptionKeywords(t *testing.T) {
	engine := NewScoringEngine()
	
	testCases := []struct {
		name        string
		description string
		keywords    []string
		expected    bool
	}{
		{
			name:        "Cannabis keyword found",
			description: "This company operates in the cannabis industry",
			keywords:    []string{"cannabis", "cbd", "marijuana"},
			expected:    true,
		},
		{
			name:        "Crypto keyword found",
			description: "We develop blockchain and cryptocurrency solutions",
			keywords:    []string{"blockchain", "crypto", "bitcoin"},
			expected:    true,
		},
		{
			name:        "No keywords found",
			description: "This is a traditional manufacturing company",
			keywords:    []string{"cannabis", "crypto", "blockchain"},
			expected:    false,
		},
		{
			name:        "Case insensitive matching",
			description: "SPAC investment vehicle",
			keywords:    []string{"spac", "blank check"},
			expected:    true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]interface{}{"description": tc.description}
			result := engine.evaluateDescriptionKeywords(data, tc.keywords)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestScoringEngine_EvaluateTransferAgent(t *testing.T) {
	engine := NewScoringEngine()
	
	testCases := []struct {
		name     string
		agent    string
		expected bool
	}{
		{
			name:     "Reputable agent",
			agent:    "Computershare Trust Company",
			expected: true,
		},
		{
			name:     "Another reputable agent",
			agent:    "Continental Stock Transfer & Trust Company",
			expected: true,
		},
		{
			name:     "Unknown agent",
			agent:    "Unknown Transfer Agent",
			expected: false,
		},
		{
			name:     "Empty agent",
			agent:    "",
			expected: false,
		},
		{
			name:     "None specified",
			agent:    "None",
			expected: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]interface{}{"transfer_agent": tc.agent}
			result := engine.evaluateActiveTransferAgent(data)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for agent: %s", tc.expected, result, tc.agent)
			}
		})
	}
}

func TestScoringEngine_DefaultModels(t *testing.T) {
	engine := NewScoringEngine()
	
	// Test Double Black Diamond model
	dbd := engine.GetDoubleBlackDiamondICP()
	if dbd.ID != "double_black_diamond" {
		t.Errorf("Expected Double Black Diamond ID, got %s", dbd.ID)
	}
	
	if len(dbd.Requirements) == 0 {
		t.Error("Expected Double Black Diamond to have requirements")
	}
	
	if len(dbd.Rules) == 0 {
		t.Error("Expected Double Black Diamond to have scoring rules")
	}
	
	// Test Pink Market model
	pink := engine.GetPinkMarketICP()
	if pink.ID != "pink_market_opportunity" {
		t.Errorf("Expected Pink Market ID, got %s", pink.ID)
	}
	
	if len(pink.Requirements) == 0 {
		t.Error("Expected Pink Market to have requirements")
	}
	
	if len(pink.Rules) == 0 {
		t.Error("Expected Pink Market to have scoring rules")
	}
	
	// Test GetAllICPModels
	allModels := engine.GetAllICPModels()
	if len(allModels) != 2 {
		t.Errorf("Expected 2 default models, got %d", len(allModels))
	}
}

func TestScoringEngine_PinkMarketScoring(t *testing.T) {
	engine := NewScoringEngine()
	
	// Test data for a Pink Market company
	companyData := map[string]interface{}{
		"ticker":           "PINK",
		"company_name":     "Pink Test Company",
		"market_tier":      "OTC Pink",
		"quote_status":     "Current Information",
		"trading_volume":   5000,
		"website":          "https://pinktest.com",
		"description":      "A legitimate pink sheet company",
		"transfer_agent":   "American Stock Transfer",
		"auditor":          "Regional CPA Firm",
		"last_10k_date":    time.Now().AddDate(0, -6, 0),  // 6 months ago
		"last_10q_date":    time.Now().AddDate(0, -2, 0),  // 2 months ago
		"last_filing_date": time.Now().AddDate(0, -1, 0),  // 1 month ago
		"profile_verified": true,
	}
	
	model := engine.GetPinkMarketICP()
	result, err := engine.ScoreCompany(companyData, model)
	if err != nil {
		t.Fatalf("Failed to score Pink Market company: %v", err)
	}
	
	// Should meet requirements (Pink market + trading volume > 0)
	if !result.RequirementsMet {
		t.Error("Expected Pink Market company to meet requirements")
	}
	
	// Should have a relatively low score due to good quality signals
	if result.Score >= 5 {
		t.Errorf("Expected low score for well-maintained Pink company, got %d", result.Score)
	}
}

// Benchmark tests
func BenchmarkScoringEngine_ScoreCompany(b *testing.B) {
	engine := NewScoringEngine()
	model := engine.GetDoubleBlackDiamondICP()
	
	companyData := map[string]interface{}{
		"ticker":           "BENCH",
		"company_name":     "Benchmark Company",
		"market_tier":      "Expert Market",
		"quote_status":     "Ineligible",
		"trading_volume":   100,
		"website":          "https://bench.com",
		"description":      "Benchmark test company",
		"last_10k_date":    time.Now().AddDate(0, -18, 0),
		"profile_verified": false,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.ScoreCompany(companyData, model)
		if err != nil {
			b.Fatalf("Scoring failed: %v", err)
		}
	}
}