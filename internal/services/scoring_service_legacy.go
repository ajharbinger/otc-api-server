package services

import (
	"database/sql"
	"fmt"
	"strings"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

// ScoringServiceLegacy is the legacy implementation for backward compatibility
type ScoringServiceLegacy struct {
	db     *sql.DB
	engine *scoring.ScoringEngine
}

// NewScoringServiceLegacy creates a new legacy scoring service
func NewScoringServiceLegacy(db *sql.DB) *ScoringServiceLegacy {
	return &ScoringServiceLegacy{
		db:     db,
		engine: scoring.NewScoringEngine(),
	}
}

// Legacy implementation of ScoringService interface
func (s *ScoringServiceLegacy) GetActiveScoringModels() ([]repository.ScoringModel, error) {
	return nil, fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) GetScoringModel(id string) (*repository.ScoringModel, error) {
	return nil, fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) CreateScoringModel(model *repository.ScoringModelForm, userID string) (*repository.ScoringModel, error) {
	return nil, fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) UpdateScoringModel(id string, model *repository.ScoringModelForm) error {
	return fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) DeleteScoringModel(id string) error {
	return fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) ScoreCompany(companyID string) error {
	return fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) ScoreCompanyWithModel(companyID, modelID string) (*repository.CompanyScore, error) {
	return nil, fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) ScoreAllCompaniesWithModel(modelID string) error {
	return fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) GetCompanyScores(companyID string) ([]repository.CompanyScore, error) {
	return nil, fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) StoreScoreResult(companyID string, result *repository.CompanyScore) error {
	return fmt.Errorf("legacy method - use new service layer")
}

func (s *ScoringServiceLegacy) getCompanyData(companyID string) (map[string]interface{}, error) {
	query := `
		SELECT ticker, company_name, market_tier, quote_status, trading_volume,
			   website, description, officers, address, transfer_agent, auditor,
			   last_10k_date, last_10q_date, last_filing_date, profile_verified
		FROM companies WHERE id = $1
	`
	
	var ticker, companyName, marketTier, quoteStatus sql.NullString
	var tradingVolume sql.NullInt64
	var website, description, officers, address, transferAgent, auditor sql.NullString
	var last10k, last10q, lastFiling sql.NullTime
	var profileVerified sql.NullBool

	err := s.db.QueryRow(query, companyID).Scan(
		&ticker, &companyName, &marketTier, &quoteStatus, &tradingVolume,
		&website, &description, &officers, &address, &transferAgent, &auditor,
		&last10k, &last10q, &lastFiling, &profileVerified,
	)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"ticker":           ticker.String,
		"company_name":     companyName.String,
		"market_tier":      marketTier.String,
		"quote_status":     quoteStatus.String,
		"trading_volume":   tradingVolume.Int64,
		"website":          website.String,
		"description":      description.String,
		"officers":         officers.String,
		"address":          address.String,
		"transfer_agent":   transferAgent.String,
		"auditor":          auditor.String,
		"profile_verified": profileVerified.Bool,
	}

	if last10k.Valid {
		data["last_10k_date"] = last10k.Time
	}
	if last10q.Valid {
		data["last_10q_date"] = last10q.Time
	}
	if lastFiling.Valid {
		data["last_filing_date"] = lastFiling.Time
	}

	return data, nil
}

func (s *ScoringServiceLegacy) containsKeywords(text string, keywords []string) bool {
	if text == "" || len(keywords) == 0 {
		return false
	}
	
	text = strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}