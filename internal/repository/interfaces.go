package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

// CompanyRepository defines the interface for company data access
type CompanyRepository interface {
	// Basic CRUD operations
	GetByID(id uuid.UUID) (*models.Company, error)
	GetByTicker(ticker string) (*models.Company, error)
	Create(company *models.Company) error
	Update(company *models.Company) error
	Delete(id uuid.UUID) error

	// Bulk operations
	GetAll(filters CompanyFilters) ([]models.Company, error)
	GetUnscored(criteria UnscoredCriteria) ([]models.Company, error)
	GetAllIDs() ([]uuid.UUID, error)
}

// ScoringRepository defines the interface for scoring data access
type ScoringRepository interface {
	// Scoring model operations
	GetActiveModels() ([]scoring.ICPModel, error)
	GetModelByID(id string) (*scoring.ICPModel, error)
	CreateModel(model *scoring.ICPModel, userID uuid.UUID) error
	UpdateModel(model *scoring.ICPModel) error
	DeleteModel(id string) error

	// Score operations
	StoreScore(score *scoring.ScoreResult) error
	GetScoresByCompany(companyID uuid.UUID) ([]scoring.ScoreResult, error)
	GetScoresByModel(modelID string) ([]scoring.ScoreResult, error)
	DeleteScoresByCompany(companyID uuid.UUID) error
	DeleteScoresByModel(modelID string) error
}

// UserRepository defines the interface for user data access
type UserRepository interface {
	GetByID(id uuid.UUID) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	Create(user *models.User) error
	Update(user *models.User) error
	Delete(id uuid.UUID) error
}

// TransactionManager defines the interface for database transaction management
type TransactionManager interface {
	WithTransaction(fn func(repos *Repositories) error) error
}

// Repositories groups all repository interfaces
type Repositories struct {
	Company CompanyRepository
	Scoring ScoringRepository
	User    UserRepository
	Tx      TransactionManager
}

// CompanyFilters defines filters for querying companies
type CompanyFilters struct {
	MarketTier    []string
	QuoteStatus   []string
	HasWebsite    *bool
	IsVerified    *bool
	MinVolume     *int64
	MaxVolume     *int64
	LastFilingFrom *time.Time
	LastFilingTo   *time.Time
	Limit         int
	Offset        int
}

// UnscoredCriteria defines criteria for finding unscored companies
type UnscoredCriteria struct {
	ModelID       string
	MarketTiers   []string
	ExcludeScored bool
	Limit         int
}