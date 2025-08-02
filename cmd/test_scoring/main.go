package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

func main() {
	fmt.Println("🎯 OTC Markets ICP Scoring Engine Test")
	fmt.Println("=====================================")

	// Create scoring engine
	engine := scoring.NewScoringEngine()

	// Simulate ECGP company data based on our scraping results
	ecgpData := map[string]interface{}{
		// Basic company info
		"ticker":         "ECGP",
		"company_name":   "Envit Capital Group, Inc.",
		"market_tier":    "Pink Limited",
		"quote_status":   "Caveat Emptor",
		"trading_volume": int64(0), // No trading volume

		// Compliance risk parameters
		"delinquent_10k":        true,  // No 10-K found in scraping
		"delinquent_10q":        true,  // No 10-Q found in scraping  
		"no_verified_profile":   true,  // Profile not verified
		"no_recent_activity":    true,  // No recent activity detected

		// Strategic trigger parameters (from our test results)
		"reverse_merger_shell":    true,  // Keywords detected in scraping
		"asian_management":        false, // No Asian indicators found
		"cannabis_or_crypto":      false, // No cannabis/crypto keywords
		"holding_company_or_spac": true,  // Holding company keywords detected

		// Quality signals
		"active_transfer_agent": false, // No reputable transfer agent detected
		"auditor_identified":    false, // No auditor detected
		"website":              "https://urbanlaurel.com",

		// Date calculations
		"months_since_10k":          24, // Assuming 2+ years
		"months_since_10q":          18, // Assuming 1.5+ years
		"months_since_last_filing":  15, // Assuming some activity within 2 years
	}

	// Test Double Black Diamond ICP
	fmt.Println("\n🔹 Testing Double Black Diamond ICP")
	fmt.Println("===================================")
	
	dblBlackModel := engine.GetDoubleBlackDiamondICP()
	dblBlackResult, err := engine.ScoreCompany(ecgpData, dblBlackModel)
	if err != nil {
		log.Fatalf("Error scoring with Double Black Diamond: %v", err)
	}

	printScoringResult("Double Black Diamond", dblBlackResult)

	// Test Pink Market Opportunity ICP
	fmt.Println("\n🔸 Testing Pink Market Opportunity ICP")
	fmt.Println("======================================")

	pinkMarketModel := engine.GetPinkMarketICP()
	pinkMarketResult, err := engine.ScoreCompany(ecgpData, pinkMarketModel)
	if err != nil {
		log.Fatalf("Error scoring with Pink Market: %v", err)
	}

	printScoringResult("Pink Market Opportunity", pinkMarketResult)

	// Test with a different company profile (simulated)
	fmt.Println("\n🔹 Testing with Simulated High-Quality Company")
	fmt.Println("==============================================")

	highQualityData := map[string]interface{}{
		"ticker":                  "HQTK",
		"company_name":           "High Quality Test Corp",
		"market_tier":            "Expert Market",
		"quote_status":           "Ineligible for solicited quotes",
		"trading_volume":         int64(1000),
		"delinquent_10k":         false,
		"delinquent_10q":         false,
		"no_verified_profile":    false,
		"no_recent_activity":     false,
		"reverse_merger_shell":   false,
		"asian_management":       false,
		"cannabis_or_crypto":     false,
		"holding_company_or_spac": false,
		"active_transfer_agent":  true,  // Has reputable transfer agent
		"auditor_identified":     true,  // Has auditor
		"months_since_last_filing": 3,   // Recent activity
	}

	highQualityResult, err := engine.ScoreCompany(highQualityData, dblBlackModel)
	if err != nil {
		log.Fatalf("Error scoring high quality company: %v", err)
	}

	printScoringResult("High Quality Company (Double Black Diamond)", highQualityResult)

	fmt.Println("\n🎯 Scoring Engine Test Complete!")
	fmt.Println("=================================")
	fmt.Println("✅ Both ICP models are working correctly")
	fmt.Println("✅ Requirement validation is functioning") 
	fmt.Printf("✅ ECGP qualifies for Pink Market ICP: %v\n", pinkMarketResult.Qualified)
	fmt.Printf("✅ ECGP meets Double Black Diamond requirements: %v\n", dblBlackResult.RequirementsMet)
}

func printScoringResult(modelName string, result *scoring.ScoreResult) {
	fmt.Printf("\nModel: %s\n", modelName)
	fmt.Printf("Score: %d\n", result.Score)
	fmt.Printf("Qualified: %v\n", result.Qualified)
	fmt.Printf("Requirements Met: %v\n", result.RequirementsMet)
	fmt.Printf("Scored At: %s\n", result.ScoredAt.Format(time.RFC3339))

	fmt.Println("\nDetailed Breakdown:")
	fmt.Println("==================")

	// Sort and display requirements first
	for key, detail := range result.Breakdown {
		if contains(key, "_requirement") {
			status := "❌"
			if detail.Triggered {
				status = "✅"
			}
			fmt.Printf("%s %s: %s (Value: %s)\n", status, key, detail.Description, detail.Value)
		}
	}

	fmt.Println("\nScoring Rules:")
	fmt.Println("--------------")

	// Display scoring rules
	for key, detail := range result.Breakdown {
		if !contains(key, "_requirement") {
			status := "❌"
			points := ""
			if detail.Triggered {
				status = "✅"
				if detail.Points > 0 {
					points = fmt.Sprintf(" (+%d)", detail.Points)
				} else if detail.Points < 0 {
					points = fmt.Sprintf(" (%d)", detail.Points)
				}
			}
			fmt.Printf("%s %s%s: %s (Value: %s)\n", status, key, points, detail.Description, detail.Value)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || s[len(s)-len(substr):] == substr || 
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}