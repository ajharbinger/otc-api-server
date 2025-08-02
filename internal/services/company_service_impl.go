package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
)

// companyServiceImpl implements CompanyService
type companyServiceImpl struct {
	repos *repository.Repositories
}

// newCompanyService creates a new company service implementation
func newCompanyService(repos *repository.Repositories) CompanyService {
	return &companyServiceImpl{
		repos: repos,
	}
}

// GetByID retrieves a company by ID
func (s *companyServiceImpl) GetByID(id string) (*repository.Company, error) {
	companyID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid company ID: %w", err)
	}

	company, err := s.repos.Company.GetByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get company: %w", err)
	}

	return s.convertFromModelsCompany(company), nil
}

// GetByTicker retrieves a company by ticker symbol
func (s *companyServiceImpl) GetByTicker(ticker string) (*repository.Company, error) {
	company, err := s.repos.Company.GetByTicker(ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to get company: %w", err)
	}

	return s.convertFromModelsCompany(company), nil
}

// GetAll retrieves companies with filters
func (s *companyServiceImpl) GetAll(filters repository.CompanyFilters) ([]repository.Company, error) {
	companies, err := s.repos.Company.GetAll(filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get companies: %w", err)
	}

	result := make([]repository.Company, len(companies))
	for i, company := range companies {
		result[i] = *s.convertFromModelsCompany(&company)
	}

	return result, nil
}

// GetUnscored retrieves companies that haven't been scored
func (s *companyServiceImpl) GetUnscored(criteria repository.UnscoredCriteria) ([]repository.Company, error) {
	companies, err := s.repos.Company.GetUnscored(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to get unscored companies: %w", err)
	}

	result := make([]repository.Company, len(companies))
	for i, company := range companies {
		result[i] = *s.convertFromModelsCompany(&company)
	}

	return result, nil
}

// Create creates a new company
func (s *companyServiceImpl) Create(company *repository.Company) error {
	modelsCompany := s.convertToModelsCompany(company)
	
	if err := s.repos.Company.Create(modelsCompany); err != nil {
		return fmt.Errorf("failed to create company: %w", err)
	}

	// Update the company with the generated ID and timestamps
	company.ID = modelsCompany.ID
	company.CreatedAt = modelsCompany.CreatedAt
	company.UpdatedAt = modelsCompany.UpdatedAt

	return nil
}

// Update updates an existing company
func (s *companyServiceImpl) Update(company *repository.Company) error {
	modelsCompany := s.convertToModelsCompany(company)
	
	if err := s.repos.Company.Update(modelsCompany); err != nil {
		return fmt.Errorf("failed to update company: %w", err)
	}

	// Update the company with the new timestamp
	company.UpdatedAt = modelsCompany.UpdatedAt

	return nil
}

// Delete deletes a company
func (s *companyServiceImpl) Delete(id string) error {
	companyID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid company ID: %w", err)
	}

	if err := s.repos.Company.Delete(companyID); err != nil {
		return fmt.Errorf("failed to delete company: %w", err)
	}

	return nil
}

// convertFromModelsCompany converts models.Company to repository.Company
func (s *companyServiceImpl) convertFromModelsCompany(company *models.Company) *repository.Company {
	// Convert Officers and Address from JSON types to strings
	var officersStr, addressStr string
	
	if company.Officers != nil {
		// Officers is already a JSON type in models, convert to string representation
		officersStr = fmt.Sprintf("%v", company.Officers)
	}
	
	if company.Address != (models.Address{}) {
		// Address is already a JSON type in models, convert to string representation
		addressStr = fmt.Sprintf("%v", company.Address)
	}

	return &repository.Company{
		ID:              company.ID,
		Ticker:          company.Ticker,
		CompanyName:     company.CompanyName,
		MarketTier:      company.MarketTier,
		QuoteStatus:     company.QuoteStatus,
		TradingVolume:   company.TradingVolume,
		Website:         company.Website,
		Description:     company.Description,
		Officers:        officersStr,
		Address:         addressStr,
		TransferAgent:   company.TransferAgent,
		Auditor:         company.Auditor,
		Last10KDate:     company.Last10KDate,
		Last10QDate:     company.Last10QDate,
		LastFilingDate:  company.LastFilingDate,
		ProfileVerified: company.ProfileVerified,
		CreatedAt:       company.CreatedAt,
		UpdatedAt:       company.UpdatedAt,
	}
}

// convertToModelsCompany converts repository.Company to models.Company
func (s *companyServiceImpl) convertToModelsCompany(company *repository.Company) *models.Company {
	// For now, set empty JSON types - in a real implementation, 
	// you would parse the JSON strings back to the proper types
	var officers models.Officers
	var address models.Address

	return &models.Company{
		ID:              company.ID,
		Ticker:          company.Ticker,
		CompanyName:     company.CompanyName,
		MarketTier:      company.MarketTier,
		QuoteStatus:     company.QuoteStatus,
		TradingVolume:   company.TradingVolume,
		Website:         company.Website,
		Description:     company.Description,
		Officers:        officers,
		Address:         address,
		TransferAgent:   company.TransferAgent,
		Auditor:         company.Auditor,
		Last10KDate:     company.Last10KDate,
		Last10QDate:     company.Last10QDate,
		LastFilingDate:  company.LastFilingDate,
		ProfileVerified: company.ProfileVerified,
		CreatedAt:       company.CreatedAt,
		UpdatedAt:       company.UpdatedAt,
	}
}