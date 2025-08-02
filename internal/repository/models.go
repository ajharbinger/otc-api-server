package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

// Company wraps the models.Company for repository layer
type Company struct {
	ID               uuid.UUID `json:"id"`
	Ticker           string    `json:"ticker"`
	CompanyName      string    `json:"company_name"`
	MarketTier       string    `json:"market_tier"`
	QuoteStatus      string    `json:"quote_status"`
	TradingVolume    int64     `json:"trading_volume"`
	Website          string    `json:"website"`
	Description      string    `json:"description"`
	Officers         string    `json:"officers"`  // JSON string
	Address          string    `json:"address"`   // JSON string
	TransferAgent    string    `json:"transfer_agent"`
	Auditor          string    `json:"auditor"`
	Last10KDate      *time.Time `json:"last_10k_date"`
	Last10QDate      *time.Time `json:"last_10q_date"`
	LastFilingDate   *time.Time `json:"last_filing_date"`
	ProfileVerified  bool      `json:"profile_verified"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ScoringModel wraps the scoring.ICPModel for repository layer
type ScoringModel struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Version      int       `json:"version"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Rules        string    `json:"rules"` // JSON string of the rules
}

// ScoringModelForm represents the form data for creating/updating scoring models
type ScoringModelForm struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Rules       string `json:"rules" binding:"required"` // JSON string
	IsActive    bool   `json:"is_active"`
}

// CompanyScore represents a company's score from a specific model
type CompanyScore struct {
	ID              uuid.UUID `json:"id"`
	CompanyID       uuid.UUID `json:"company_id"`
	ScoringModelID  string    `json:"scoring_model_id"`
	Score           int       `json:"score"`
	Qualified       bool      `json:"qualified"`
	RequirementsMet bool      `json:"requirements_met"`
	Breakdown       string    `json:"breakdown"` // JSON string
	ScoredAt        time.Time `json:"scored_at"`
	ModelName       string    `json:"model_name,omitempty"`
}

// LoginResponse represents the response from login
type LoginResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refresh_token"`
	User         models.User `json:"user"`
	ExpiresAt    time.Time   `json:"expires_at"`
}

// RegisterRequest represents the request to register a new user
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role,omitempty"`
}

// ConvertToICPModel converts a ScoringModel to scoring.ICPModel
func (sm *ScoringModel) ConvertToICPModel() (*scoring.ICPModel, error) {
	engine := scoring.NewScoringEngine()
	return engine.LoadICPModelFromJSON(
		sm.ID, sm.Name, sm.Description, sm.Version, 
		[]byte(sm.Rules), sm.IsActive, sm.CreatedAt, sm.UpdatedAt,
	)
}

// ConvertFromICPModel converts a scoring.ICPModel to ScoringModel
func ConvertFromICPModel(model *scoring.ICPModel, rules string) *ScoringModel {
	return &ScoringModel{
		ID:          model.ID,
		Name:        model.Name,
		Description: model.Description,
		Version:     model.Version,
		IsActive:    model.IsActive,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		Rules:       rules,
	}
}