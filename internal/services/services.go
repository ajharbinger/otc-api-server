package services

import (
	"database/sql"

	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

// Services contains all application services
type Services struct {
	Company CompanyService
	Scoring ScoringService
	Auth    AuthService
}

// CompanyService defines the interface for company business logic
type CompanyService interface {
	GetByID(id string) (*repository.Company, error)
	GetByTicker(ticker string) (*repository.Company, error)
	GetAll(filters repository.CompanyFilters) ([]repository.Company, error)
	GetUnscored(criteria repository.UnscoredCriteria) ([]repository.Company, error)
	Create(company *repository.Company) error
	Update(company *repository.Company) error
	Delete(id string) error
}

// ScoringService defines the interface for scoring business logic
type ScoringService interface {
	// Model management
	GetActiveScoringModels() ([]repository.ScoringModel, error)
	GetScoringModel(id string) (*repository.ScoringModel, error)
	CreateScoringModel(model *repository.ScoringModelForm, userID string) (*repository.ScoringModel, error)
	UpdateScoringModel(id string, model *repository.ScoringModelForm) error
	DeleteScoringModel(id string) error

	// Scoring operations
	ScoreCompany(companyID string) error
	ScoreCompanyWithModel(companyID, modelID string) (*repository.CompanyScore, error)
	ScoreAllCompaniesWithModel(modelID string) error
	GetCompanyScores(companyID string) ([]repository.CompanyScore, error)
	StoreScoreResult(companyID string, result *repository.CompanyScore) error
}

// AuthService defines the interface for authentication business logic
type AuthService interface {
	Login(email, password string) (*repository.LoginResponse, error)
	Register(user *repository.RegisterRequest) (*models.User, error)
	ValidateToken(token string) (*models.User, error)
	RefreshToken(token string) (*repository.LoginResponse, error)
}

// NewServices creates a new Services instance with all dependencies
func NewServices(db *sql.DB, cfg *config.Config) *Services {
	repos := repository.NewRepositories(db)
	
	return &Services{
		Company: newCompanyService(repos),
		Scoring: newScoringService(repos),
		Auth:    newAuthService(repos, cfg),
	}
}

// NewScoringService creates a standalone scoring service
func NewScoringService(repos *repository.Repositories) ScoringService {
	return newScoringService(repos)
}