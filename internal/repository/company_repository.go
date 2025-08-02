package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
)

// companyRepository implements CompanyRepository
type companyRepository struct {
	db dbExecutor
}

// NewCompanyRepository creates a new company repository
func NewCompanyRepository(db dbExecutor) CompanyRepository {
	return &companyRepository{db: db}
}

// GetByID retrieves a company by ID
func (r *companyRepository) GetByID(id uuid.UUID) (*models.Company, error) {
	query := `
		SELECT id, ticker, company_name, market_tier, quote_status, trading_volume,
			   website, description, officers, address, transfer_agent, auditor,
			   last_10k_date, last_10q_date, last_filing_date, profile_verified,
			   created_at, updated_at
		FROM companies WHERE id = $1
	`
	
	company := &models.Company{}
	err := r.db.QueryRow(query, id).Scan(
		&company.ID, &company.Ticker, &company.CompanyName, &company.MarketTier,
		&company.QuoteStatus, &company.TradingVolume, &company.Website,
		&company.Description, &company.Officers, &company.Address,
		&company.TransferAgent, &company.Auditor, &company.Last10KDate,
		&company.Last10QDate, &company.LastFilingDate, &company.ProfileVerified,
		&company.CreatedAt, &company.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("company not found")
		}
		return nil, fmt.Errorf("failed to get company: %w", err)
	}
	
	return company, nil
}

// GetByTicker retrieves a company by ticker symbol
func (r *companyRepository) GetByTicker(ticker string) (*models.Company, error) {
	query := `
		SELECT id, ticker, company_name, market_tier, quote_status, trading_volume,
			   website, description, officers, address, transfer_agent, auditor,
			   last_10k_date, last_10q_date, last_filing_date, profile_verified,
			   created_at, updated_at
		FROM companies WHERE ticker = $1
	`
	
	company := &models.Company{}
	err := r.db.QueryRow(query, ticker).Scan(
		&company.ID, &company.Ticker, &company.CompanyName, &company.MarketTier,
		&company.QuoteStatus, &company.TradingVolume, &company.Website,
		&company.Description, &company.Officers, &company.Address,
		&company.TransferAgent, &company.Auditor, &company.Last10KDate,
		&company.Last10QDate, &company.LastFilingDate, &company.ProfileVerified,
		&company.CreatedAt, &company.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("company with ticker %s not found", ticker)
		}
		return nil, fmt.Errorf("failed to get company: %w", err)
	}
	
	return company, nil
}

// Create creates a new company
func (r *companyRepository) Create(company *models.Company) error {
	if company.ID == uuid.Nil {
		company.ID = uuid.New()
	}
	
	now := time.Now()
	company.CreatedAt = now
	company.UpdatedAt = now
	
	query := `
		INSERT INTO companies (
			id, ticker, company_name, market_tier, quote_status, trading_volume,
			website, description, officers, address, transfer_agent, auditor,
			last_10k_date, last_10q_date, last_filing_date, profile_verified,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)
	`
	
	_, err := r.db.Exec(query,
		company.ID, company.Ticker, company.CompanyName, company.MarketTier,
		company.QuoteStatus, company.TradingVolume, company.Website,
		company.Description, company.Officers, company.Address,
		company.TransferAgent, company.Auditor, company.Last10KDate,
		company.Last10QDate, company.LastFilingDate, company.ProfileVerified,
		company.CreatedAt, company.UpdatedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create company: %w", err)
	}
	
	return nil
}

// Update updates an existing company
func (r *companyRepository) Update(company *models.Company) error {
	company.UpdatedAt = time.Now()
	
	query := `
		UPDATE companies SET
			company_name = $2, market_tier = $3, quote_status = $4, trading_volume = $5,
			website = $6, description = $7, officers = $8, address = $9,
			transfer_agent = $10, auditor = $11, last_10k_date = $12,
			last_10q_date = $13, last_filing_date = $14, profile_verified = $15,
			updated_at = $16
		WHERE id = $1
	`
	
	result, err := r.db.Exec(query,
		company.ID, company.CompanyName, company.MarketTier, company.QuoteStatus,
		company.TradingVolume, company.Website, company.Description,
		company.Officers, company.Address, company.TransferAgent, company.Auditor,
		company.Last10KDate, company.Last10QDate, company.LastFilingDate,
		company.ProfileVerified, company.UpdatedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update company: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("company not found")
	}
	
	return nil
}

// Delete deletes a company
func (r *companyRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM companies WHERE id = $1`
	
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete company: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("company not found")
	}
	
	return nil
}

// GetAll retrieves companies with filters
func (r *companyRepository) GetAll(filters CompanyFilters) ([]models.Company, error) {
	query := `
		SELECT id, ticker, company_name, market_tier, quote_status, trading_volume,
			   website, description, officers, address, transfer_agent, auditor,
			   last_10k_date, last_10q_date, last_filing_date, profile_verified,
			   created_at, updated_at
		FROM companies
	`
	
	var whereClauses []string
	var args []interface{}
	argIndex := 1
	
	// Apply filters
	if len(filters.MarketTier) > 0 {
		placeholders := make([]string, len(filters.MarketTier))
		for i, tier := range filters.MarketTier {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, tier)
			argIndex++
		}
		whereClauses = append(whereClauses, fmt.Sprintf("market_tier IN (%s)", strings.Join(placeholders, ",")))
	}
	
	if len(filters.QuoteStatus) > 0 {
		placeholders := make([]string, len(filters.QuoteStatus))
		for i, status := range filters.QuoteStatus {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, status)
			argIndex++
		}
		whereClauses = append(whereClauses, fmt.Sprintf("quote_status IN (%s)", strings.Join(placeholders, ",")))
	}
	
	if filters.HasWebsite != nil {
		if *filters.HasWebsite {
			whereClauses = append(whereClauses, "website IS NOT NULL AND website != ''")
		} else {
			whereClauses = append(whereClauses, "(website IS NULL OR website = '')")
		}
	}
	
	if filters.IsVerified != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("profile_verified = $%d", argIndex))
		args = append(args, *filters.IsVerified)
		argIndex++
	}
	
	if filters.MinVolume != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("trading_volume >= $%d", argIndex))
		args = append(args, *filters.MinVolume)
		argIndex++
	}
	
	if filters.MaxVolume != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("trading_volume <= $%d", argIndex))
		args = append(args, *filters.MaxVolume)
		argIndex++
	}
	
	if filters.LastFilingFrom != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("last_filing_date >= $%d", argIndex))
		args = append(args, *filters.LastFilingFrom)
		argIndex++
	}
	
	if filters.LastFilingTo != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("last_filing_date <= $%d", argIndex))
		args = append(args, *filters.LastFilingTo)
		argIndex++
	}
	
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	
	query += " ORDER BY updated_at DESC"
	
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filters.Limit)
		argIndex++
	}
	
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filters.Offset)
	}
	
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query companies: %w", err)
	}
	defer rows.Close()
	
	var companies []models.Company
	for rows.Next() {
		var company models.Company
		err := rows.Scan(
			&company.ID, &company.Ticker, &company.CompanyName, &company.MarketTier,
			&company.QuoteStatus, &company.TradingVolume, &company.Website,
			&company.Description, &company.Officers, &company.Address,
			&company.TransferAgent, &company.Auditor, &company.Last10KDate,
			&company.Last10QDate, &company.LastFilingDate, &company.ProfileVerified,
			&company.CreatedAt, &company.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan company: %w", err)
		}
		companies = append(companies, company)
	}
	
	return companies, nil
}

// GetUnscored retrieves companies that haven't been scored by a specific model
func (r *companyRepository) GetUnscored(criteria UnscoredCriteria) ([]models.Company, error) {
	query := `
		SELECT c.id, c.ticker, c.company_name, c.market_tier, c.quote_status, c.trading_volume,
			   c.website, c.description, c.officers, c.address, c.transfer_agent, c.auditor,
			   c.last_10k_date, c.last_10q_date, c.last_filing_date, c.profile_verified,
			   c.created_at, c.updated_at
		FROM companies c
	`
	
	var whereClauses []string
	var args []interface{}
	argIndex := 1
	
	if criteria.ExcludeScored {
		query += " LEFT JOIN company_scores cs ON c.id = cs.company_id AND cs.scoring_model_id = $1"
		args = append(args, criteria.ModelID)
		whereClauses = append(whereClauses, "cs.company_id IS NULL")
		argIndex++
	}
	
	if len(criteria.MarketTiers) > 0 {
		placeholders := make([]string, len(criteria.MarketTiers))
		for i, tier := range criteria.MarketTiers {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			args = append(args, tier)
			argIndex++
		}
		whereClauses = append(whereClauses, fmt.Sprintf("c.market_tier IN (%s)", strings.Join(placeholders, ",")))
	}
	
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	
	query += " ORDER BY c.updated_at DESC"
	
	if criteria.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, criteria.Limit)
	}
	
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query unscored companies: %w", err)
	}
	defer rows.Close()
	
	var companies []models.Company
	for rows.Next() {
		var company models.Company
		err := rows.Scan(
			&company.ID, &company.Ticker, &company.CompanyName, &company.MarketTier,
			&company.QuoteStatus, &company.TradingVolume, &company.Website,
			&company.Description, &company.Officers, &company.Address,
			&company.TransferAgent, &company.Auditor, &company.Last10KDate,
			&company.Last10QDate, &company.LastFilingDate, &company.ProfileVerified,
			&company.CreatedAt, &company.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan company: %w", err)
		}
		companies = append(companies, company)
	}
	
	return companies, nil
}

// GetAllIDs retrieves all company IDs
func (r *companyRepository) GetAllIDs() ([]uuid.UUID, error) {
	query := `SELECT id FROM companies ORDER BY updated_at DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query company IDs: %w", err)
	}
	defer rows.Close()
	
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan company ID: %w", err)
		}
		ids = append(ids, id)
	}
	
	return ids, nil
}