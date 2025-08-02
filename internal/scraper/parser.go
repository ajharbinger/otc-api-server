package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
)

// Parser handles parsing of OTC Markets pages
type Parser struct{}

// NewParser creates a new parser instance
func NewParser() *Parser {
	return &Parser{}
}

// ParseOverviewPage extracts data from the overview page
func (p *Parser) ParseOverviewPage(doc *goquery.Document) map[string]interface{} {
	data := make(map[string]interface{})

	// Extract company name from title as fallback
	if title := doc.Find("title").Text(); title != "" {
		parts := strings.Split(title, " - ")
		if len(parts) >= 2 {
			// Format: "ECGP - Envit Capital Group, Inc. | Overview | OTC Markets"
			companyInfo := strings.TrimSpace(parts[1])
			companyInfo = strings.Split(companyInfo, " | ")[0] // Remove "| Overview | OTC Markets"
			data["company_name"] = companyInfo
			
			// Extract ticker from first part
			ticker := strings.TrimSpace(parts[0])
			data["ticker"] = ticker
		}
	}

	// Try to find market tier information - OTC Markets uses various selectors
	marketTierSelectors := []string{
		"[data-testid*='tier']",
		".market-tier",
		"[class*='tier']",
		"[class*='market']",
		".otc-tier",
		"span:contains('Pink')",
		"span:contains('Expert')",
		"span:contains('OTCQX')",
		"span:contains('OTCQB')",
	}
	
	for _, selector := range marketTierSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if p.isMarketTier(text) {
				data["market_tier"] = text
			}
		})
	}

	// Try to find quote status
	quoteStatusSelectors := []string{
		"[data-testid*='quote']",
		".quote-status",
		"[class*='quote']",
		"span:contains('Caveat Emptor')",
		"span:contains('Ineligible')",
		"span:contains('Limited Information')",
	}
	
	for _, selector := range quoteStatusSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if p.isQuoteStatus(text) {
				data["quote_status"] = text
			}
		})
	}

	// Extract all text content and search for patterns
	allText := doc.Find("body").Text()
	
	// Look for market tier patterns in all text
	tierPatterns := []string{
		`Pink\s+Limited`,
		`Pink\s+Market`,
		`Expert\s+Market`,
		`OTCQX`,
		`OTCQB`,
		`Grey\s+Market`,
	}
	
	for _, pattern := range tierPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		if matches := re.FindString(allText); matches != "" {
			data["market_tier"] = strings.TrimSpace(matches)
			break
		}
	}

	// Look for quote status patterns
	quotePatterns := []string{
		`Caveat\s+Emptor`,
		`Ineligible\s+for\s+solicited\s+quotes`,
		`Limited\s+Information`,
		`Current\s+Information`,
		`Adequate\s+Information`,
	}
	
	for _, pattern := range quotePatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		if matches := re.FindString(allText); matches != "" {
			data["quote_status"] = strings.TrimSpace(matches)
			break
		}
	}

	// Look for trading volume patterns
	volumePattern := regexp.MustCompile(`(?i)volume[:\s]+([0-9,]+)`)
	if matches := volumePattern.FindStringSubmatch(allText); len(matches) > 1 {
		if volume := p.parseVolume(matches[1]); volume > 0 {
			data["trading_volume"] = volume
		}
	}

	// Look for website patterns
	websitePattern := regexp.MustCompile(`(?i)website[:\s]+([a-zA-Z0-9\.\-]+\.[a-zA-Z]{2,})`)
	if matches := websitePattern.FindStringSubmatch(allText); len(matches) > 1 {
		data["website"] = "https://" + matches[1]
	}

	// Extract business description for keyword analysis
	p.extractBusinessDescription(doc, data, allText)
	
	// Extract officer information for geographic analysis
	p.extractOfficerInfo(doc, data, allText)
	
	// Extract transfer agent information
	p.extractTransferAgent(doc, data, allText)

	// Try generic selectors for common data
	p.extractGenericData(doc, data)

	return data
}

// ParseFinancialsPage extracts data from the financials page
func (p *Parser) ParseFinancialsPage(doc *goquery.Document) map[string]interface{} {
	data := make(map[string]interface{})

	// Look for filing dates in the full text
	allText := doc.Find("body").Text()
	
	// Enhanced 10-K filing date patterns
	tenKPatterns := []string{
		`(?i)10-?K[:\s]*filed[:\s]*([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
		`(?i)10-?K[:\s]*([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
		`(?i)annual\s+report[:\s]*([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
		`(?i)form\s+10-?K[:\s]*([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
	}
	
	for _, pattern := range tenKPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(allText); len(matches) > 1 {
			if date := p.parseDate(matches[1]); date != nil {
				data["last_10k_date"] = date
				break
			}
		}
	}

	// Enhanced 10-Q filing date patterns
	tenQPatterns := []string{
		`(?i)10-?Q[:\s]*filed[:\s]*([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
		`(?i)10-?Q[:\s]*([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
		`(?i)quarterly\s+report[:\s]*([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
		`(?i)form\s+10-?Q[:\s]*([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
	}
	
	for _, pattern := range tenQPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(allText); len(matches) > 1 {
			if date := p.parseDate(matches[1]); date != nil {
				data["last_10q_date"] = date
				break
			}
		}
	}
	
	// Calculate delinquency flags for scoring
	now := time.Now()
	if tenKDate, ok := data["last_10k_date"].(*time.Time); ok && tenKDate != nil {
		monthsSince := int(now.Sub(*tenKDate).Hours() / 24 / 30.44) // Average month length
		data["delinquent_10k"] = monthsSince > 15
		data["months_since_10k"] = monthsSince
	} else {
		data["delinquent_10k"] = true // No 10-K found
	}
	
	if tenQDate, ok := data["last_10q_date"].(*time.Time); ok && tenQDate != nil {
		monthsSince := int(now.Sub(*tenQDate).Hours() / 24 / 30.44)
		data["delinquent_10q"] = monthsSince > 6
		data["months_since_10q"] = monthsSince
	} else {
		data["delinquent_10q"] = true // No 10-Q found
	}

	// Look for auditor information
	auditorPatterns := []string{
		`(?i)auditor[:\s]+([A-Za-z\s&,]+(?:LLP|LLC|CPA|PC))`,
		`(?i)independent\s+auditor[:\s]+([A-Za-z\s&,]+(?:LLP|LLC|CPA|PC))`,
	}
	
	for _, pattern := range auditorPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(allText); len(matches) > 1 {
			data["auditor"] = strings.TrimSpace(matches[1])
			break
		}
	}

	// Try to find filing tables or lists
	doc.Find("table, ul, ol").Each(func(i int, s *goquery.Selection) {
		text := strings.ToLower(s.Text())
		if strings.Contains(text, "10-k") || strings.Contains(text, "10-q") || strings.Contains(text, "filing") {
			p.extractFilingDatesFromElement(s, data)
		}
	})

	return data
}

// ParseDisclosurePage extracts data from the disclosure page
func (p *Parser) ParseDisclosurePage(doc *goquery.Document) map[string]interface{} {
	data := make(map[string]interface{})

	allText := doc.Find("body").Text()

	// Enhanced profile verification status detection
	data["profile_verified"] = false // Default to false
	verifiedPatterns := []string{
		`(?i)profile\s+verified`,
		`(?i)verified\s+profile`,
		`(?i)profile\s+status[:\s]+verified`,
		`(?i)company\s+profile[:\s]*verified`,
		`(?i)verification[:\s]*complete`,
	}
	
	notVerifiedPatterns := []string{
		`(?i)profile\s+not\s+verified`,
		`(?i)unverified\s+profile`,
		`(?i)profile\s+status[:\s]+not\s+verified`,
		`(?i)verification[:\s]*pending`,
	}
	
	for _, pattern := range verifiedPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(allText) {
			data["profile_verified"] = true
			break
		}
	}
	
	for _, pattern := range notVerifiedPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(allText) {
			data["profile_verified"] = false
			data["no_verified_profile"] = true
			break
		}
	}

	// Look for latest filing dates
	datePatterns := []string{
		`([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4})`,
		`([A-Za-z]+\s+[0-9]{1,2},?\s+[0-9]{4})`,
	}
	
	var latestDate *time.Time
	for _, pattern := range datePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(allText, -1)
		for _, match := range matches {
			if date := p.parseDate(match); date != nil {
				if latestDate == nil || date.After(*latestDate) {
					latestDate = date
				}
			}
		}
	}
	
	if latestDate != nil {
		data["last_filing_date"] = latestDate
		
		// Check for recent activity (within last 12 months)
		now := time.Now()
		monthsSince := int(now.Sub(*latestDate).Hours() / 24 / 30.44)
		data["months_since_last_filing"] = monthsSince
		data["no_recent_activity"] = monthsSince > 12
	} else {
		data["no_recent_activity"] = true // No activity found
	}

	return data
}

// Helper methods

// isMarketTier checks if text represents a market tier
func (p *Parser) isMarketTier(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	marketTiers := []string{
		"pink limited",
		"pink market",
		"expert market",
		"otcqx",
		"otcqb",
		"grey market",
		"gray market",
	}
	
	for _, tier := range marketTiers {
		if strings.Contains(text, tier) {
			return true
		}
	}
	return false
}

// isQuoteStatus checks if text represents a quote status
func (p *Parser) isQuoteStatus(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	quoteStatuses := []string{
		"caveat emptor",
		"ineligible",
		"limited information",
		"current information",
		"adequate information",
		"no information",
	}
	
	for _, status := range quoteStatuses {
		if strings.Contains(text, status) {
			return true
		}
	}
	return false
}

// extractGenericData tries to extract data using common patterns
func (p *Parser) extractGenericData(doc *goquery.Document, data map[string]interface{}) {
	// Look for any elements that might contain structured data
	doc.Find("[data-testid], [class*='data'], [class*='info'], [id*='data'], [id*='info']").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" && len(text) < 200 { // Avoid very long texts
			// Try to identify what this might be
			lowerText := strings.ToLower(text)
			
			if strings.Contains(lowerText, "volume") && regexp.MustCompile(`[0-9,]+`).MatchString(text) {
				if volume := p.parseVolume(text); volume > 0 {
					data["trading_volume"] = volume
				}
			}
			
			if strings.Contains(lowerText, "website") || strings.Contains(lowerText, "www.") {
				if website := p.extractWebsite(text); website != "" {
					data["website"] = website
				}
			}
		}
	})
}

// extractWebsite tries to extract a website URL from text
func (p *Parser) extractWebsite(text string) string {
	websitePattern := regexp.MustCompile(`(?i)(https?://[a-zA-Z0-9\.\-/]+|www\.[a-zA-Z0-9\.\-/]+|[a-zA-Z0-9\.\-]+\.(com|org|net|co|io))`)
	if matches := websitePattern.FindString(text); matches != "" {
		if !strings.HasPrefix(matches, "http") {
			return "https://" + matches
		}
		return matches
	}
	return ""
}

// extractFilingDatesFromElement extracts filing dates from a table or list element
func (p *Parser) extractFilingDatesFromElement(s *goquery.Selection, data map[string]interface{}) {
	s.Find("td, li, div").Each(func(i int, cell *goquery.Selection) {
		text := strings.TrimSpace(cell.Text())
		if strings.Contains(strings.ToLower(text), "10-k") {
			if date := p.extractDateFromText(text); date != nil {
				data["last_10k_date"] = date
			}
		}
		if strings.Contains(strings.ToLower(text), "10-q") {
			if date := p.extractDateFromText(text); date != nil {
				data["last_10q_date"] = date
			}
		}
	})
}

// extractDateFromText finds and parses a date from text
func (p *Parser) extractDateFromText(text string) *time.Time {
	datePattern := regexp.MustCompile(`([0-9]{1,2}[/\-][0-9]{1,2}[/\-][0-9]{2,4}|[A-Za-z]+\s+[0-9]{1,2},?\s+[0-9]{4})`)
	if matches := datePattern.FindString(text); matches != "" {
		return p.parseDate(matches)
	}
	return nil
}

// parseVolume extracts numeric volume from text
func (p *Parser) parseVolume(volumeText string) int64 {
	// Remove common formatting and extract numbers
	re := regexp.MustCompile(`[\d,]+`)
	numStr := re.FindString(strings.ReplaceAll(volumeText, ",", ""))
	
	if num, err := strconv.ParseInt(numStr, 10, 64); err == nil {
		return num
	}
	
	return 0
}

// parseDate attempts to parse various date formats commonly found on OTC Markets
func (p *Parser) parseDate(dateText string) *time.Time {
	dateText = strings.TrimSpace(dateText)
	if dateText == "" {
		return nil
	}

	// Common date formats found on OTC Markets
	formats := []string{
		"01/02/2006",
		"1/2/2006",
		"01-02-2006",
		"1-2-2006",
		"2006-01-02",
		"January 2, 2006",
		"Jan 2, 2006",
		"January 2 2006",
		"Jan 2 2006",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateText); err == nil {
			return &date
		}
	}

	return nil
}

// extractBusinessDescription extracts company description and analyzes for strategic triggers
func (p *Parser) extractBusinessDescription(doc *goquery.Document, data map[string]interface{}, allText string) {
	// Look for business description in common locations
	descriptionSelectors := []string{
		"[class*='description']",
		"[class*='business']",
		"[class*='overview']",
		".company-description",
		"#business-description",
		"p:contains('business')",
	}
	
	description := ""
	for _, selector := range descriptionSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if len(text) > len(description) && len(text) > 50 {
				description = text
			}
		})
	}
	
	// Fall back to extracting description patterns from full text
	if description == "" {
		descPatterns := []string{
			`(?i)business[:\s]+([^.]{50,300})`,
			`(?i)company\s+engages?\s+in[:\s]*([^.]{50,300})`,
			`(?i)principal\s+business[:\s]+([^.]{50,300})`,
		}
		
		for _, pattern := range descPatterns {
			re := regexp.MustCompile(pattern)
			if matches := re.FindStringSubmatch(allText); len(matches) > 1 {
				description = strings.TrimSpace(matches[1])
				break
			}
		}
	}
	
	if description != "" {
		data["description"] = description
		
		// Analyze description for strategic triggers
		lowerDesc := strings.ToLower(description)
		
		// Reverse merger/shell detection
		shellKeywords := []string{"reverse merger", "shell company", "blank check", "business combination", "acquisition vehicle"}
		for _, keyword := range shellKeywords {
			if strings.Contains(lowerDesc, keyword) {
				data["reverse_merger_shell"] = true
				break
			}
		}
		
		// Cannabis/crypto detection
		cannabisCryptoKeywords := []string{"cannabis", "cbd", "marijuana", "hemp", "blockchain", "cryptocurrency", "bitcoin", "crypto", "digital currency"}
		for _, keyword := range cannabisCryptoKeywords {
			if strings.Contains(lowerDesc, keyword) {
				data["cannabis_or_crypto"] = true
				break
			}
		}
		
		// Holding company/SPAC detection
		holdingKeywords := []string{"holding company", "spac", "special purpose acquisition", "investment company", "capital company"}
		for _, keyword := range holdingKeywords {
			if strings.Contains(lowerDesc, keyword) {
				data["holding_company_or_spac"] = true
				break
			}
		}
	}
}

// extractOfficerInfo extracts officer information and analyzes for geographic patterns
func (p *Parser) extractOfficerInfo(doc *goquery.Document, data map[string]interface{}, allText string) {
	officers := make([]map[string]interface{}, 0)
	
	// Look for officer tables or sections
	doc.Find("table, div, section").Each(func(i int, s *goquery.Selection) {
		text := strings.ToLower(s.Text())
		if strings.Contains(text, "officer") || strings.Contains(text, "director") || strings.Contains(text, "management") {
			// Extract officer information from this section
			s.Find("tr, div, p").Each(func(j int, row *goquery.Selection) {
				rowText := strings.TrimSpace(row.Text())
				if len(rowText) > 10 && (strings.Contains(strings.ToLower(rowText), "ceo") || 
					strings.Contains(strings.ToLower(rowText), "president") ||
					strings.Contains(strings.ToLower(rowText), "director")) {
					
					officer := map[string]interface{}{
						"info": rowText,
					}
					officers = append(officers, officer)
				}
			})
		}
	})
	
	if len(officers) > 0 {
		data["officers"] = officers
		
		// Analyze for Asian management (strategic trigger)
		asianIndicators := []string{"taiwan", "hong kong", "china", "singapore", "tw", "hk", "cn", "beijing", "shanghai", "taipei"}
		allOfficerText := strings.ToLower(fmt.Sprintf("%v", officers))
		
		for _, indicator := range asianIndicators {
			if strings.Contains(allOfficerText, indicator) {
				data["asian_management"] = true
				break
			}
		}
	}
}

// extractTransferAgent extracts transfer agent information
func (p *Parser) extractTransferAgent(doc *goquery.Document, data map[string]interface{}, allText string) {
	transferAgentPatterns := []string{
		`(?i)transfer\s+agent[:\s]+([A-Za-z\s&,\.]+(?:LLC|Inc|Corp|Company))`,
		`(?i)registrar[:\s]+([A-Za-z\s&,\.]+(?:LLC|Inc|Corp|Company))`,
		`(?i)stock\s+transfer[:\s]+([A-Za-z\s&,\.]+(?:LLC|Inc|Corp|Company))`,
	}
	
	for _, pattern := range transferAgentPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(allText); len(matches) > 1 {
			transferAgent := strings.TrimSpace(matches[1])
			data["transfer_agent"] = transferAgent
			
			// Check if it's a reputable transfer agent (quality signal)
			reputableAgents := []string{"computershare", "continental", "american stock", "vstock", "pacific stock"}
			lowerAgent := strings.ToLower(transferAgent)
			
			for _, reputable := range reputableAgents {
				if strings.Contains(lowerAgent, reputable) {
					data["active_transfer_agent"] = true
					break
				}
			}
			break
		}
	}
}