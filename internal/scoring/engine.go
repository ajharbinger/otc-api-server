package scoring

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ScoringEngine handles ICP-based company scoring
type ScoringEngine struct{}

// NewScoringEngine creates a new scoring engine instance
func NewScoringEngine() *ScoringEngine {
	return &ScoringEngine{}
}

// ScoringRule represents a single scoring rule
type ScoringRule struct {
	Field       string      `json:"field"`
	Operator    string      `json:"operator"`
	Value       interface{} `json:"value"`
	Weight      int         `json:"weight"`
	Description string      `json:"description"`
}

// ICPModel represents an Ideal Customer Profile scoring model
type ICPModel struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Version      int           `json:"version"`
	Requirements []Requirement `json:"must_have"`
	Exclusions   []Requirement `json:"must_not"`
	Rules        []ScoringRule `json:"scoring_rules"`
	MinScore     int           `json:"minimum_score"`
	IsActive     bool          `json:"is_active"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// Requirement represents a mandatory requirement for an ICP
type Requirement struct {
	Field       string      `json:"field"`
	Operator    string      `json:"operator"`
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
}

// ScoreResult represents the result of scoring a company
type ScoreResult struct {
	CompanyID       string                 `json:"company_id"`
	ScoringModelID  string                 `json:"scoring_model_id"`
	Score           int                    `json:"score"`
	Qualified       bool                   `json:"qualified"`
	RequirementsMet bool                   `json:"requirements_met"`
	Breakdown       map[string]ScoreDetail `json:"breakdown"`
	ScoredAt        time.Time              `json:"scored_at"`
}

// ScoreDetail provides detailed information about a scoring component
type ScoreDetail struct {
	Points      int    `json:"points"`
	Triggered   bool   `json:"triggered"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

// ScoreCompany scores a company against a specific ICP model
func (e *ScoringEngine) ScoreCompany(companyData map[string]interface{}, model ICPModel) (*ScoreResult, error) {
	result := &ScoreResult{
		ScoringModelID:  model.ID,
		Score:           0,
		Qualified:       false,
		RequirementsMet: true,
		Breakdown:       make(map[string]ScoreDetail),
		ScoredAt:        time.Now(),
	}

	// Check mandatory requirements first
	for _, req := range model.Requirements {
		met, value := e.evaluateCondition(companyData, req.Field, req.Operator, req.Value)
		if !met {
			result.RequirementsMet = false
			result.Breakdown[req.Field+"_requirement"] = ScoreDetail{
				Points:      0,
				Triggered:   false,
				Description: fmt.Sprintf("REQUIREMENT: %s", req.Description),
				Value:       fmt.Sprintf("%v", value),
			}
		} else {
			result.Breakdown[req.Field+"_requirement"] = ScoreDetail{
				Points:      0,
				Triggered:   true,
				Description: fmt.Sprintf("REQUIREMENT MET: %s", req.Description),
				Value:       fmt.Sprintf("%v", value),
			}
		}
	}

	// Check exclusions (must NOT have)
	for _, exclusion := range model.Exclusions {
		met, value := e.evaluateCondition(companyData, exclusion.Field, exclusion.Operator, exclusion.Value)
		if met {
			result.RequirementsMet = false
			result.Breakdown[exclusion.Field+"_exclusion"] = ScoreDetail{
				Points:      0,
				Triggered:   true,
				Description: fmt.Sprintf("EXCLUSION VIOLATED: %s", exclusion.Description),
				Value:       fmt.Sprintf("%v", value),
			}
		} else {
			result.Breakdown[exclusion.Field+"_exclusion"] = ScoreDetail{
				Points:      0,
				Triggered:   false,
				Description: fmt.Sprintf("EXCLUSION OK: %s", exclusion.Description),
				Value:       fmt.Sprintf("%v", value),
			}
		}
	}

	// Only calculate score if requirements are met
	if result.RequirementsMet {
		// Apply scoring rules
		for _, rule := range model.Rules {
			triggered, value := e.evaluateCondition(companyData, rule.Field, rule.Operator, rule.Value)
			
			detail := ScoreDetail{
				Points:      0,
				Triggered:   triggered,
				Description: rule.Description,
				Value:       fmt.Sprintf("%v", value),
			}
			
			if triggered {
				detail.Points = rule.Weight
				result.Score += rule.Weight
			}
			
			result.Breakdown[rule.Field] = detail
		}

		// Check if company qualifies based on minimum score
		result.Qualified = result.Score >= model.MinScore
	}

	return result, nil
}

// LoadICPModelFromJSON loads an ICP model from JSON data (from database)
func (e *ScoringEngine) LoadICPModelFromJSON(id, name, description string, version int, rulesJSON []byte, isActive bool, createdAt, updatedAt time.Time) (*ICPModel, error) {
	var rules map[string]interface{}
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules JSON: %w", err)
	}

	model := &ICPModel{
		ID:          id,
		Name:        name,
		Description: description,
		Version:     version,
		IsActive:    isActive,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	// Parse must_have requirements
	if mustHave, exists := rules["must_have"]; exists {
		if mustHaveSlice, ok := mustHave.([]interface{}); ok {
			for _, item := range mustHaveSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					req := Requirement{
						Field:       getString(itemMap, "field"),
						Operator:    getString(itemMap, "operator"),
						Value:       itemMap["value"],
						Description: getString(itemMap, "description"),
					}
					model.Requirements = append(model.Requirements, req)
				}
			}
		}
	}

	// Parse must_not exclusions
	if mustNot, exists := rules["must_not"]; exists {
		if mustNotSlice, ok := mustNot.([]interface{}); ok {
			for _, item := range mustNotSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					req := Requirement{
						Field:       getString(itemMap, "field"),
						Operator:    getString(itemMap, "operator"),
						Value:       itemMap["value"],
						Description: getString(itemMap, "description"),
					}
					model.Exclusions = append(model.Exclusions, req)
				}
			}
		}
	}

	// Parse scoring rules
	if scoringRules, exists := rules["scoring_rules"]; exists {
		if rulesSlice, ok := scoringRules.([]interface{}); ok {
			for _, item := range rulesSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					rule := ScoringRule{
						Field:       getString(itemMap, "field"),
						Operator:    getString(itemMap, "operator"),
						Value:       itemMap["value"],
						Weight:      getInt(itemMap, "weight"),
						Description: getString(itemMap, "description"),
					}
					// Handle legacy condition field
					if condition := getString(itemMap, "condition"); condition != "" {
						rule.Description = condition
					}
					model.Rules = append(model.Rules, rule)
				}
			}
		}
	}

	// Parse minimum score
	if minScore, exists := rules["minimum_score"]; exists {
		model.MinScore = getInt(map[string]interface{}{"minimum_score": minScore}, "minimum_score")
	}

	return model, nil
}

// Helper functions for JSON parsing
func getString(m map[string]interface{}, key string) string {
	if val, exists := m[key]; exists {
		return fmt.Sprintf("%v", val)
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, exists := m[key]; exists {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return 0
}

// GetDefaultICPModels returns the default ICP models as defined in the PRD
func (e *ScoringEngine) GetDefaultICPModels() []*ICPModel {
	return []*ICPModel{
		{
			ID:          "double-black-diamond",
			Name:        "Double Black Diamond",
			Description: "Companies in Expert Market needing services to regain eligibility",
			Version:     1,
			Requirements: []Requirement{
				{Field: "market_tier", Operator: "equals", Value: "Expert Market", Description: "Must be in Expert Market tier"},
				{Field: "quote_status", Operator: "contains", Value: "Ineligible", Description: "Must be ineligible for solicited quotes"},
			},
			Rules: []ScoringRule{
				{Field: "delinquent_10k", Weight: 1, Description: "Delinquent 10-K filing (>15 months)"},
				{Field: "delinquent_10q", Weight: 1, Description: "Delinquent 10-Q filing (>6 months)"},
				{Field: "no_verified_profile", Weight: 1, Description: "Profile not verified"},
				{Field: "pink_limited_or_expert", Weight: 1, Description: "In risky market tier"},
				{Field: "no_recent_activity", Weight: 1, Description: "No recent activity (>12 months)"},
				{Field: "reverse_merger_shell", Weight: 1, Description: "Reverse merger or shell company indicators"},
				{Field: "asian_management", Weight: 1, Description: "Asian management team"},
				{Field: "cannabis_or_crypto", Weight: 1, Description: "Cannabis or crypto business"},
				{Field: "holding_company_or_spac", Weight: 1, Description: "Holding company or SPAC"},
				{Field: "active_transfer_agent", Weight: -1, Description: "Has active transfer agent"},
				{Field: "domain_linked_to_company", Weight: -1, Description: "Website matches company name"},
				{Field: "auditor_identified", Weight: -1, Description: "Has identified auditor"},
			},
			MinScore: 3,
			IsActive: true,
		},
		{
			ID:          "pink-market-opportunity",
			Name:        "Pink Market Opportunity",
			Description: "Active Pink sheet companies with potential compliance gaps",
			Version:     1,
			Requirements: []Requirement{
				{Field: "market_tier", Operator: "equals", Value: "OTC Pink", Description: "Must be in OTC Pink tier"},
				{Field: "trading_volume", Operator: "greater_than", Value: 0, Description: "Must have trading volume"},
			},
			Exclusions: []Requirement{
				{Field: "last_filing_date", Operator: "less_than", Value: "2023-01-01", Description: "Must not have filings older than 2023"},
			},
			Rules: []ScoringRule{
				{Field: "delinquent_10k", Weight: 1, Description: "Delinquent 10-K filing (>15 months)"},
				{Field: "delinquent_10q", Weight: 1, Description: "Delinquent 10-Q filing (>6 months)"},
				{Field: "no_verified_profile", Weight: 1, Description: "Profile not verified"},
				{Field: "no_recent_activity", Weight: 1, Description: "No recent activity (>12 months)"},
				{Field: "reverse_merger_shell", Weight: 1, Description: "Reverse merger or shell company indicators"},
				{Field: "asian_management", Weight: 1, Description: "Asian management team"},
				{Field: "cannabis_or_crypto", Weight: 1, Description: "Cannabis or crypto business"},
				{Field: "holding_company_or_spac", Weight: 1, Description: "Holding company or SPAC"},
				{Field: "active_transfer_agent", Weight: -1, Description: "Has active transfer agent"},
				{Field: "domain_linked_to_company", Weight: -1, Description: "Website matches company name"},
				{Field: "auditor_identified", Weight: -1, Description: "Has identified auditor"},
			},
			MinScore: 3,
			IsActive: true,
		},
	}
}

// evaluateCondition evaluates a scoring condition against company data
func (e *ScoringEngine) evaluateCondition(data map[string]interface{}, field, operator string, expectedValue interface{}) (bool, interface{}) {
	// Handle special computed fields
	switch field {
	case "delinquent_10k":
		return e.evaluateDelinquency(data, "last_10k_date", 15), data["last_10k_date"]
	case "delinquent_10q":
		return e.evaluateDelinquency(data, "last_10q_date", 6), data["last_10q_date"]
	case "no_recent_activity":
		return e.evaluateDelinquency(data, "last_filing_date", 12), data["last_filing_date"]
	case "pink_limited_or_expert":
		return e.evaluateMarketTierRisk(data), data["market_tier"]
	case "reverse_merger_shell":
		return e.evaluateDescriptionKeywords(data, []string{"reverse merger", "shell company", "shell corporation"}), data["description"]
	case "asian_management":
		return e.evaluateAsianManagement(data), e.getOfficerLocations(data)
	case "cannabis_or_crypto":
		return e.evaluateDescriptionKeywords(data, []string{"cannabis", "cbd", "marijuana", "blockchain", "crypto", "bitcoin"}), data["description"]
	case "holding_company_or_spac":
		return e.evaluateDescriptionKeywords(data, []string{"blank check", "spac", "holding company", "special purpose"}), data["description"]
	case "active_transfer_agent":
		return e.evaluateActiveTransferAgent(data), data["transfer_agent"]
	case "domain_linked_to_company":
		return e.evaluateDomainMatch(data), data["website"]
	case "auditor_identified":
		return e.evaluateAuditorPresent(data), data["auditor"]
	case "no_verified_profile":
		// Invert profile_verified for scoring
		if verified, ok := data["profile_verified"].(bool); ok {
			return !verified, verified
		}
		return true, false // Default to not verified
	}

	// Standard field evaluation
	actualValue, exists := data[field]
	if !exists {
		return false, nil
	}

	switch operator {
	case "equals":
		return fmt.Sprintf("%v", actualValue) == fmt.Sprintf("%v", expectedValue), actualValue
	case "not_equals":
		return fmt.Sprintf("%v", actualValue) != fmt.Sprintf("%v", expectedValue), actualValue
	case "contains":
		actualStr := strings.ToLower(fmt.Sprintf("%v", actualValue))
		expectedStr := strings.ToLower(fmt.Sprintf("%v", expectedValue))
		return strings.Contains(actualStr, expectedStr), actualValue
	case "greater_than":
		return e.compareNumbers(actualValue, expectedValue, ">"), actualValue
	case "less_than":
		return e.compareNumbers(actualValue, expectedValue, "<"), actualValue
	case "greater_than_or_equal":
		return e.compareNumbers(actualValue, expectedValue, ">="), actualValue
	case "less_than_or_equal":
		return e.compareNumbers(actualValue, expectedValue, "<="), actualValue
	case "is_true":
		if boolVal, ok := actualValue.(bool); ok {
			return boolVal, actualValue
		}
		return fmt.Sprintf("%v", actualValue) == "true", actualValue
	case "is_false":
		if boolVal, ok := actualValue.(bool); ok {
			return !boolVal, actualValue
		}
		return fmt.Sprintf("%v", actualValue) == "false", actualValue
	case "in":
		return e.evaluateInList(actualValue, expectedValue), actualValue
	case "not_in":
		return !e.evaluateInList(actualValue, expectedValue), actualValue
	case "regex":
		return e.evaluateRegex(actualValue, expectedValue), actualValue
	default:
		return false, actualValue
	}
}

// evaluateDelinquency checks if a date field indicates delinquency
func (e *ScoringEngine) evaluateDelinquency(data map[string]interface{}, dateField string, monthsThreshold int) bool {
	dateValue, exists := data[dateField]
	if !exists || dateValue == nil {
		return true // No date means delinquent
	}

	var lastDate time.Time
	switch v := dateValue.(type) {
	case time.Time:
		lastDate = v
	case *time.Time:
		if v == nil {
			return true
		}
		lastDate = *v
	case string:
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			return true // Unparseable date means delinquent
		}
		lastDate = parsed
	default:
		return true // Unknown format means delinquent
	}

	monthsSince := time.Since(lastDate).Hours() / 24 / 30.44
	return monthsSince > float64(monthsThreshold)
}

// evaluateMarketTierRisk checks if company is in risky market tiers
func (e *ScoringEngine) evaluateMarketTierRisk(data map[string]interface{}) bool {
	tier, exists := data["market_tier"]
	if !exists {
		return false
	}

	tierStr := strings.ToLower(fmt.Sprintf("%v", tier))
	riskTiers := []string{"pink limited", "expert market", "grey market", "gray market"}
	
	for _, riskTier := range riskTiers {
		if strings.Contains(tierStr, riskTier) {
			return true
		}
	}
	return false
}

// evaluateDescriptionKeywords checks for keywords in business description
func (e *ScoringEngine) evaluateDescriptionKeywords(data map[string]interface{}, keywords []string) bool {
	description, exists := data["description"]
	if !exists || description == nil {
		return false
	}

	descStr := strings.ToLower(fmt.Sprintf("%v", description))
	for _, keyword := range keywords {
		if strings.Contains(descStr, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// evaluateAsianManagement checks for Asian management indicators
func (e *ScoringEngine) evaluateAsianManagement(data map[string]interface{}) bool {
	// Check officers data
	officers, exists := data["officers"]
	if exists && officers != nil {
		if e.containsAsianLocations(fmt.Sprintf("%v", officers)) {
			return true
		}
	}

	// Check company address
	address, exists := data["address"]
	if exists && address != nil {
		if e.containsAsianLocations(fmt.Sprintf("%v", address)) {
			return true
		}
	}

	return false
}

// containsAsianLocations checks for Asian country/region indicators
func (e *ScoringEngine) containsAsianLocations(text string) bool {
	text = strings.ToLower(text)
	indicators := []string{
		"taiwan", "tw", "hong kong", "hk", "china", "cn", "singapore", "sg",
		"beijing", "shanghai", "shenzhen", "taipei", "macau", "mo",
	}
	
	for _, indicator := range indicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}
	return false
}

// getOfficerLocations extracts officer location information for display
func (e *ScoringEngine) getOfficerLocations(data map[string]interface{}) interface{} {
	officers, exists := data["officers"]
	if !exists {
		return "No officer data"
	}
	return officers
}

// evaluateActiveTransferAgent checks for reputable transfer agents
func (e *ScoringEngine) evaluateActiveTransferAgent(data map[string]interface{}) bool {
	agent, exists := data["transfer_agent"]
	if !exists || agent == nil {
		return false
	}

	agentStr := strings.ToLower(fmt.Sprintf("%v", agent))
	if len(strings.TrimSpace(agentStr)) == 0 {
		return false
	}

	// Check for reputable transfer agents
	reputableAgents := []string{
		"computershare", "continental", "american stock", "island stock", 
		"vstock", "pacific stock", "securities transfer", "registrar and transfer",
	}
	
	for _, reputable := range reputableAgents {
		if strings.Contains(agentStr, reputable) {
			return true
		}
	}

	// If it has any transfer agent that's not obviously fake, consider it active
	return !strings.Contains(agentStr, "unknown") && !strings.Contains(agentStr, "none")
}

// evaluateDomainMatch checks if website domain matches company name
func (e *ScoringEngine) evaluateDomainMatch(data map[string]interface{}) bool {
	website, hasWebsite := data["website"]
	companyName, hasName := data["company_name"]
	
	if !hasWebsite || !hasName || website == nil || companyName == nil {
		return false
	}

	websiteStr := strings.ToLower(fmt.Sprintf("%v", website))
	nameStr := strings.ToLower(fmt.Sprintf("%v", companyName))
	
	// Extract domain from website
	if strings.Contains(websiteStr, "//") {
		parts := strings.Split(websiteStr, "//")
		if len(parts) > 1 {
			websiteStr = parts[1]
		}
	}
	if strings.Contains(websiteStr, "/") {
		websiteStr = strings.Split(websiteStr, "/")[0]
	}
	if strings.HasPrefix(websiteStr, "www.") {
		websiteStr = websiteStr[4:]
	}
	
	// Clean company name for comparison
	nameWords := strings.Fields(strings.ReplaceAll(nameStr, ",", " "))
	
	// Check if any significant words from company name appear in domain
	for _, word := range nameWords {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 3 && word != "inc" && word != "corp" && word != "company" && word != "ltd" {
			if strings.Contains(websiteStr, word) {
				return true
			}
		}
	}
	
	return false
}

// evaluateAuditorPresent checks if company has an identified auditor
func (e *ScoringEngine) evaluateAuditorPresent(data map[string]interface{}) bool {
	auditor, exists := data["auditor"]
	if !exists || auditor == nil {
		return false
	}

	auditorStr := strings.TrimSpace(fmt.Sprintf("%v", auditor))
	return len(auditorStr) > 0 && 
		   !strings.Contains(strings.ToLower(auditorStr), "unknown") &&
		   !strings.Contains(strings.ToLower(auditorStr), "none")
}

// evaluateInList checks if value is in a list
func (e *ScoringEngine) evaluateInList(actual, expected interface{}) bool {
	actualStr := fmt.Sprintf("%v", actual)
	
	switch v := expected.(type) {
	case []interface{}:
		for _, item := range v {
			if fmt.Sprintf("%v", item) == actualStr {
				return true
			}
		}
	case []string:
		for _, item := range v {
			if item == actualStr {
				return true
			}
		}
	case string:
		// Handle comma-separated list
		items := strings.Split(v, ",")
		for _, item := range items {
			if strings.TrimSpace(item) == actualStr {
				return true
			}
		}
	}
	return false
}

// evaluateRegex checks if value matches regex pattern
func (e *ScoringEngine) evaluateRegex(actual, expected interface{}) bool {
	actualStr := fmt.Sprintf("%v", actual)
	regexStr := fmt.Sprintf("%v", expected)
	
	matched, err := regexp.MatchString(regexStr, actualStr)
	return err == nil && matched
}

// compareNumbers compares numeric values
func (e *ScoringEngine) compareNumbers(actual, expected interface{}, operator string) bool {
	actualFloat, actualOK := e.toFloat64(actual)
	expectedFloat, expectedOK := e.toFloat64(expected)
	
	if !actualOK || !expectedOK {
		return false
	}

	switch operator {
	case ">":
		return actualFloat > expectedFloat
	case "<":
		return actualFloat < expectedFloat
	case ">=":
		return actualFloat >= expectedFloat
	case "<=":
		return actualFloat <= expectedFloat
	default:
		return false
	}
}

// toFloat64 converts interface{} to float64
func (e *ScoringEngine) toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

// GetDoubleBlackDiamondICP returns the Double Black Diamond ICP model
func (e *ScoringEngine) GetDoubleBlackDiamondICP() ICPModel {
	return ICPModel{
		ID:          "double_black_diamond",
		Name:        "Double Black Diamond",
		Description: "Find companies in the highest-risk tier who need services to regain eligibility",
		Version:     1,
		Requirements: []Requirement{
			{
				Field:       "market_tier",
				Operator:    "contains",
				Value:       "Expert Market",
				Description: "Market Tier must be Expert Market (⚫⚫)",
			},
			{
				Field:       "quote_status",
				Operator:    "contains",
				Value:       "Ineligible",
				Description: "Quote Status must be 'Ineligible for solicited quotes'",
			},
		},
		Rules: []ScoringRule{
			// Compliance Risk Parameters (+1 each)
			{
				Field:       "delinquent_10k",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "No 10-K filing in last 15 months",
			},
			{
				Field:       "delinquent_10q",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "No 10-Q filing in last 6 months",
			},
			{
				Field:       "no_verified_profile",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Profile not verified on OTC Markets",
			},
			{
				Field:       "no_recent_activity",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "No news/filings in last 12 months",
			},
			// Strategic Trigger Parameters (+1 each)
			{
				Field:       "reverse_merger_shell",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Business description suggests reverse merger or shell company",
			},
			{
				Field:       "asian_management",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Officers or address in Taiwan, Hong Kong, or China",
			},
			{
				Field:       "cannabis_or_crypto",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Business involves cannabis, CBD, blockchain, or cryptocurrency",
			},
			{
				Field:       "holding_company_or_spac",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Company is a holding company, SPAC, or investment vehicle",
			},
			// Quality Signals (-1 each, reduces risk score)
			{
				Field:       "active_transfer_agent",
				Operator:    "is_true",
				Value:       true,
				Weight:      -1,
				Description: "Has reputable transfer agent (reduces risk)",
			},
			{
				Field:       "auditor_identified",
				Operator:    "is_true",
				Value:       true,
				Weight:      -1,
				Description: "Has identified CPA firm (reduces risk)",
			},
		},
		MinScore: 3,
	}
}

// GetPinkMarketICP returns the Pink Market Opportunity ICP model
func (e *ScoringEngine) GetPinkMarketICP() ICPModel {
	return ICPModel{
		ID:          "pink_market_opportunity",
		Name:        "Pink Market Opportunity",
		Description: "Find companies in Pink sheets that are active but may have compliance gaps",
		Version:     1,
		Requirements: []Requirement{
			{
				Field:       "market_tier",
				Operator:    "contains",
				Value:       "Pink",
				Description: "Market Tier must be OTC Pink",
			},
			{
				Field:       "trading_volume",
				Operator:    "greater_than",
				Value:       0,
				Description: "Must have trading volume > 0",
			},
		},
		Rules: []ScoringRule{
			// Filing recency check (not older than 2023)
			{
				Field:       "months_since_last_filing",
				Operator:    "less_than",
				Value:       24, // Less than 2 years (since 2023)
				Weight:      2,
				Description: "Recent filing activity (since 2023)",
			},
			// Same compliance risk parameters as Double Black Diamond
			{
				Field:       "delinquent_10k",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "No 10-K filing in last 15 months",
			},
			{
				Field:       "delinquent_10q",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "No 10-Q filing in last 6 months",
			},
			{
				Field:       "no_verified_profile",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Profile not verified on OTC Markets",
			},
			{
				Field:       "no_recent_activity",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "No news/filings in last 12 months",
			},
			// Strategic triggers
			{
				Field:       "reverse_merger_shell",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Business description suggests reverse merger or shell company",
			},
			{
				Field:       "asian_management",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Officers or address in Asia",
			},
			{
				Field:       "cannabis_or_crypto",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Cannabis or crypto business",
			},
			{
				Field:       "holding_company_or_spac",
				Operator:    "is_true",
				Value:       true,
				Weight:      1,
				Description: "Holding company or SPAC structure",
			},
			// Quality signals (reduce score)
			{
				Field:       "active_transfer_agent",
				Operator:    "is_true",
				Value:       true,
				Weight:      -1,
				Description: "Has reputable transfer agent",
			},
			{
				Field:       "auditor_identified",
				Operator:    "is_true",
				Value:       true,
				Weight:      -1,
				Description: "Has identified auditor",
			},
		},
		MinScore: 3,
	}
}

// GetAllICPModels returns all available ICP models
func (e *ScoringEngine) GetAllICPModels() []ICPModel {
	return []ICPModel{
		e.GetDoubleBlackDiamondICP(),
		e.GetPinkMarketICP(),
	}
}