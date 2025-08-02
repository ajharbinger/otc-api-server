package services

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

// LeadExportService handles filtering and exporting qualified companies
type LeadExportService struct {
	db             *sql.DB
	scoringService ScoringService
}

// NewLeadExportService creates a new lead export service
func NewLeadExportService(db *sql.DB, scoringService ScoringService) *LeadExportService {
	return &LeadExportService{
		db:             db,
		scoringService: scoringService,
	}
}

// LeadFilter contains filtering criteria for qualified companies
type LeadFilter struct {
	ModelIDs             []string  `json:"model_ids"`              // ICP models to include
	MinScore             *int      `json:"min_score"`              // Minimum score threshold
	MaxScore             *int      `json:"max_score"`              // Maximum score threshold
	MarketTiers          []string  `json:"market_tiers"`           // Market tiers to include
	QuoteStatuses        []string  `json:"quote_statuses"`         // Quote statuses to include
	ScoredAfter          *time.Time `json:"scored_after"`          // Only scores after this date
	ScoredBefore         *time.Time `json:"scored_before"`         // Only scores before this date
	TradingVolumeMin     *int64    `json:"trading_volume_min"`     // Minimum trading volume
	TradingVolumeMax     *int64    `json:"trading_volume_max"`     // Maximum trading volume
	HasWebsite           *bool     `json:"has_website"`            // Filter by website presence
	HasTransferAgent     *bool     `json:"has_transfer_agent"`     // Filter by transfer agent presence
	HasAuditor           *bool     `json:"has_auditor"`            // Filter by auditor presence
	IncludeRequiredOnly  bool      `json:"include_required_only"`  // Only companies meeting requirements
	ExcludeFields        []string  `json:"exclude_fields"`         // Fields to exclude from export
	Limit                *int      `json:"limit"`                  // Limit number of results
}

// ExportFormat specifies the format for exporting leads
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
)

// LeadExportOptions contains options for exporting leads
type LeadExportOptions struct {
	Format       ExportFormat `json:"format"`
	IncludeScoreBreakdown bool  `json:"include_score_breakdown"`
	IncludeMetadata      bool  `json:"include_metadata"`
}

// QualifiedLead represents a company qualified for outreach
type QualifiedLead struct {
	// Company Information
	ID              string    `json:"id" csv:"id"`
	Ticker          string    `json:"ticker" csv:"ticker"`
	CompanyName     string    `json:"company_name" csv:"company_name"`
	MarketTier      string    `json:"market_tier" csv:"market_tier"`
	QuoteStatus     string    `json:"quote_status" csv:"quote_status"`
	TradingVolume   *int64    `json:"trading_volume" csv:"trading_volume"`
	Website         *string   `json:"website" csv:"website"`
	Description     *string   `json:"description" csv:"description"`
	
	// Contact Information
	Officers        *string   `json:"officers" csv:"officers"`
	Address         *string   `json:"address" csv:"address"`
	TransferAgent   *string   `json:"transfer_agent" csv:"transfer_agent"`
	Auditor         *string   `json:"auditor" csv:"auditor"`
	
	// Filing Information
	Last10KDate     *time.Time `json:"last_10k_date" csv:"last_10k_date"`
	Last10QDate     *time.Time `json:"last_10q_date" csv:"last_10q_date"`
	LastFilingDate  *time.Time `json:"last_filing_date" csv:"last_filing_date"`
	ProfileVerified *bool     `json:"profile_verified" csv:"profile_verified"`
	
	// Scoring Information
	ModelID         string                     `json:"model_id" csv:"model_id"`
	ModelName       string                     `json:"model_name" csv:"model_name"`
	Score           int                        `json:"score" csv:"score"`
	Qualified       bool                       `json:"qualified" csv:"qualified"`
	RequirementsMet bool                       `json:"requirements_met" csv:"requirements_met"`
	ScoreBreakdown  map[string]scoring.ScoreDetail `json:"score_breakdown,omitempty" csv:"-"`
	ScoredAt        time.Time                  `json:"scored_at" csv:"scored_at"`
	
	// Business Insights
	RiskIndicators  []string  `json:"risk_indicators" csv:"risk_indicators"`
	Opportunities   []string  `json:"opportunities" csv:"opportunities"`
	RecommendedServices []string `json:"recommended_services" csv:"recommended_services"`
}

// GetQualifiedLeads retrieves companies that match the filtering criteria
func (s *LeadExportService) GetQualifiedLeads(filter LeadFilter) ([]QualifiedLead, error) {
	query, args := s.buildFilterQuery(filter)
	
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute filter query: %w", err)
	}
	defer rows.Close()

	var leads []QualifiedLead
	for rows.Next() {
		lead, err := s.scanQualifiedLead(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan qualified lead: %w", err)
		}
		
		// Add business insights
		s.addBusinessInsights(&lead)
		
		leads = append(leads, lead)
	}

	return leads, nil
}

// ExportQualifiedLeads exports qualified leads in the specified format
func (s *LeadExportService) ExportQualifiedLeads(filter LeadFilter, options LeadExportOptions) ([]byte, error) {
	leads, err := s.GetQualifiedLeads(filter)
	if err != nil {
		return nil, err
	}

	switch options.Format {
	case FormatJSON:
		return s.exportToJSON(leads, options)
	case FormatCSV:
		return s.exportToCSV(leads, options)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", options.Format)
	}
}

// buildFilterQuery constructs the SQL query based on filter criteria
func (s *LeadExportService) buildFilterQuery(filter LeadFilter) (string, []interface{}) {
	baseQuery := `
		SELECT 
			c.id, c.ticker, c.company_name, c.market_tier, c.quote_status,
			c.trading_volume, c.website, c.description, c.officers, c.address,
			c.transfer_agent, c.auditor, c.last_10k_date, c.last_10q_date,
			c.last_filing_date, c.profile_verified,
			cs.scoring_model_id, sm.name as model_name, cs.score,
			cs.score_breakdown, cs.scored_at
		FROM companies c
		JOIN company_scores cs ON c.id = cs.company_id
		JOIN scoring_models sm ON cs.scoring_model_id = sm.id
		WHERE 1=1
	`

	var conditions []string
	var args []interface{}
	argIndex := 1

	// Filter by model IDs
	if len(filter.ModelIDs) > 0 {
		placeholders := make([]string, len(filter.ModelIDs))
		for i, modelID := range filter.ModelIDs {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, modelID)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("cs.scoring_model_id IN (%s)", strings.Join(placeholders, ",")))
	}

	// Filter by score range
	if filter.MinScore != nil {
		conditions = append(conditions, fmt.Sprintf("cs.score >= $%d", argIndex))
		args = append(args, *filter.MinScore)
		argIndex++
	}

	if filter.MaxScore != nil {
		conditions = append(conditions, fmt.Sprintf("cs.score <= $%d", argIndex))
		args = append(args, *filter.MaxScore)
		argIndex++
	}

	// Filter by market tiers
	if len(filter.MarketTiers) > 0 {
		placeholders := make([]string, len(filter.MarketTiers))
		for i, tier := range filter.MarketTiers {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, tier)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("c.market_tier IN (%s)", strings.Join(placeholders, ",")))
	}

	// Filter by quote statuses
	if len(filter.QuoteStatuses) > 0 {
		placeholders := make([]string, len(filter.QuoteStatuses))
		for i, status := range filter.QuoteStatuses {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, status)
			argIndex++
		}
		conditions = append(conditions, fmt.Sprintf("c.quote_status IN (%s)", strings.Join(placeholders, ",")))
	}

	// Filter by scoring date range
	if filter.ScoredAfter != nil {
		conditions = append(conditions, fmt.Sprintf("cs.scored_at >= $%d", argIndex))
		args = append(args, *filter.ScoredAfter)
		argIndex++
	}

	if filter.ScoredBefore != nil {
		conditions = append(conditions, fmt.Sprintf("cs.scored_at <= $%d", argIndex))
		args = append(args, *filter.ScoredBefore)
		argIndex++
	}

	// Filter by trading volume
	if filter.TradingVolumeMin != nil {
		conditions = append(conditions, fmt.Sprintf("c.trading_volume >= $%d", argIndex))
		args = append(args, *filter.TradingVolumeMin)
		argIndex++
	}

	if filter.TradingVolumeMax != nil {
		conditions = append(conditions, fmt.Sprintf("c.trading_volume <= $%d", argIndex))
		args = append(args, *filter.TradingVolumeMax)
		argIndex++
	}

	// Filter by website presence
	if filter.HasWebsite != nil {
		if *filter.HasWebsite {
			conditions = append(conditions, "c.website IS NOT NULL AND c.website != ''")
		} else {
			conditions = append(conditions, "(c.website IS NULL OR c.website = '')")
		}
	}

	// Filter by transfer agent presence
	if filter.HasTransferAgent != nil {
		if *filter.HasTransferAgent {
			conditions = append(conditions, "c.transfer_agent IS NOT NULL AND c.transfer_agent != ''")
		} else {
			conditions = append(conditions, "(c.transfer_agent IS NULL OR c.transfer_agent = '')")
		}
	}

	// Filter by auditor presence
	if filter.HasAuditor != nil {
		if *filter.HasAuditor {
			conditions = append(conditions, "c.auditor IS NOT NULL AND c.auditor != ''")
		} else {
			conditions = append(conditions, "(c.auditor IS NULL OR c.auditor = '')")
		}
	}

	// Include requirements met filter if requested
	if filter.IncludeRequiredOnly {
		// This would require parsing the score breakdown JSON
		// For now, we'll use a simple score threshold
		conditions = append(conditions, "cs.score >= 3")
	}

	// Build final query
	query := baseQuery
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY cs.score DESC, c.ticker ASC"

	// Add limit if specified
	if filter.Limit != nil {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, *filter.Limit)
	}

	return query, args
}

// scanQualifiedLead scans a database row into a QualifiedLead struct
func (s *LeadExportService) scanQualifiedLead(rows *sql.Rows) (QualifiedLead, error) {
	var lead QualifiedLead
	var breakdownJSON string
	var officers, address sql.NullString
	var tradingVolume sql.NullInt64
	var website, description, transferAgent, auditor sql.NullString
	var last10K, last10Q, lastFiling sql.NullTime
	var profileVerified sql.NullBool

	err := rows.Scan(
		&lead.ID, &lead.Ticker, &lead.CompanyName, &lead.MarketTier, &lead.QuoteStatus,
		&tradingVolume, &website, &description, &officers, &address,
		&transferAgent, &auditor, &last10K, &last10Q, &lastFiling, &profileVerified,
		&lead.ModelID, &lead.ModelName, &lead.Score, &breakdownJSON, &lead.ScoredAt,
	)
	if err != nil {
		return lead, err
	}

	// Handle nullable fields
	if tradingVolume.Valid {
		lead.TradingVolume = &tradingVolume.Int64
	}
	if website.Valid {
		lead.Website = &website.String
	}
	if description.Valid {
		lead.Description = &description.String
	}
	if officers.Valid {
		lead.Officers = &officers.String
	}
	if address.Valid {
		lead.Address = &address.String
	}
	if transferAgent.Valid {
		lead.TransferAgent = &transferAgent.String
	}
	if auditor.Valid {
		lead.Auditor = &auditor.String
	}
	if last10K.Valid {
		lead.Last10KDate = &last10K.Time
	}
	if last10Q.Valid {
		lead.Last10QDate = &last10Q.Time
	}
	if lastFiling.Valid {
		lead.LastFilingDate = &lastFiling.Time
	}
	if profileVerified.Valid {
		lead.ProfileVerified = &profileVerified.Bool
	}

	// Parse score breakdown
	if err := json.Unmarshal([]byte(breakdownJSON), &lead.ScoreBreakdown); err != nil {
		return lead, fmt.Errorf("failed to unmarshal score breakdown: %w", err)
	}

	// Determine qualification status from breakdown
	lead.Qualified = lead.Score >= 3 // Minimum threshold
	lead.RequirementsMet = true // Assume requirements met if scored

	return lead, nil
}

// addBusinessInsights adds business insights and recommendations to a lead
func (s *LeadExportService) addBusinessInsights(lead *QualifiedLead) {
	var riskIndicators []string
	var opportunities []string
	var services []string

	// Analyze score breakdown for insights
	for field, detail := range lead.ScoreBreakdown {
		if detail.Triggered && detail.Points > 0 {
			switch field {
			case "delinquent_10k":
				riskIndicators = append(riskIndicators, "Delinquent 10-K filing")
				services = append(services, "SEC Filing Services")
			case "delinquent_10q":
				riskIndicators = append(riskIndicators, "Delinquent 10-Q filing")
				services = append(services, "Quarterly Reporting Services")
			case "no_verified_profile":
				riskIndicators = append(riskIndicators, "Unverified OTC profile")
				services = append(services, "OTC Profile Verification")
			case "reverse_merger_shell":
				riskIndicators = append(riskIndicators, "Shell company structure")
				services = append(services, "Corporate Restructuring Services")
			case "cannabis_or_crypto":
				riskIndicators = append(riskIndicators, "High-risk industry")
				services = append(services, "Compliance Advisory Services")
			case "holding_company_or_spac":
				opportunities = append(opportunities, "Investment vehicle structure")
				services = append(services, "M&A Advisory Services")
			}
		}
	}

	// Market tier specific insights
	switch lead.MarketTier {
	case "Expert Market":
		riskIndicators = append(riskIndicators, "Highest risk tier")
		services = append(services, "Market Tier Upgrade Services")
	case "Pink Limited":
		riskIndicators = append(riskIndicators, "Limited information available")
		services = append(services, "Information Enhancement Services")
	}

	// Trading volume insights
	if lead.TradingVolume != nil && *lead.TradingVolume == 0 {
		riskIndicators = append(riskIndicators, "No trading activity")
		services = append(services, "Market Making Services")
	}

	lead.RiskIndicators = riskIndicators
	lead.Opportunities = opportunities
	lead.RecommendedServices = services
}

// exportToJSON exports leads to JSON format
func (s *LeadExportService) exportToJSON(leads []QualifiedLead, options LeadExportOptions) ([]byte, error) {
	if !options.IncludeScoreBreakdown {
		// Remove score breakdown from each lead
		for i := range leads {
			leads[i].ScoreBreakdown = nil
		}
	}

	exportData := map[string]interface{}{
		"leads": leads,
		"count": len(leads),
		"exported_at": time.Now(),
	}

	if options.IncludeMetadata {
		exportData["metadata"] = map[string]interface{}{
			"export_format": "json",
			"include_score_breakdown": options.IncludeScoreBreakdown,
			"total_companies": len(leads),
		}
	}

	return json.MarshalIndent(exportData, "", "  ")
}

// exportToCSV exports leads to CSV format
func (s *LeadExportService) exportToCSV(leads []QualifiedLead, options LeadExportOptions) ([]byte, error) {
	var output strings.Builder
	writer := csv.NewWriter(&output)

	// Write header
	headers := []string{
		"id", "ticker", "company_name", "market_tier", "quote_status",
		"trading_volume", "website", "description", "officers", "address",
		"transfer_agent", "auditor", "last_10k_date", "last_10q_date",
		"last_filing_date", "profile_verified", "model_id", "model_name",
		"score", "qualified", "requirements_met", "scored_at",
		"risk_indicators", "opportunities", "recommended_services",
	}

	if err := writer.Write(headers); err != nil {
		return nil, err
	}

	// Write data rows
	for _, lead := range leads {
		row := []string{
			lead.ID,
			lead.Ticker,
			lead.CompanyName,
			lead.MarketTier,
			lead.QuoteStatus,
			s.formatNullInt64(lead.TradingVolume),
			s.formatNullString(lead.Website),
			s.formatNullString(lead.Description),
			s.formatNullString(lead.Officers),
			s.formatNullString(lead.Address),
			s.formatNullString(lead.TransferAgent),
			s.formatNullString(lead.Auditor),
			s.formatNullTime(lead.Last10KDate),
			s.formatNullTime(lead.Last10QDate),
			s.formatNullTime(lead.LastFilingDate),
			s.formatNullBool(lead.ProfileVerified),
			lead.ModelID,
			lead.ModelName,
			strconv.Itoa(lead.Score),
			strconv.FormatBool(lead.Qualified),
			strconv.FormatBool(lead.RequirementsMet),
			lead.ScoredAt.Format(time.RFC3339),
			strings.Join(lead.RiskIndicators, "; "),
			strings.Join(lead.Opportunities, "; "),
			strings.Join(lead.RecommendedServices, "; "),
		}

		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return []byte(output.String()), nil
}

// Helper functions for CSV formatting
func (s *LeadExportService) formatNullString(val *string) string {
	if val == nil {
		return ""
	}
	return *val
}

func (s *LeadExportService) formatNullInt64(val *int64) string {
	if val == nil {
		return ""
	}
	return strconv.FormatInt(*val, 10)
}

func (s *LeadExportService) formatNullTime(val *time.Time) string {
	if val == nil {
		return ""
	}
	return val.Format("2006-01-02")
}

func (s *LeadExportService) formatNullBool(val *bool) string {
	if val == nil {
		return ""
	}
	return strconv.FormatBool(*val)
}