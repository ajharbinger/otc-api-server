package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

// scoringRepository implements ScoringRepository
type scoringRepository struct {
	db     dbExecutor
	engine *scoring.ScoringEngine
}

// NewScoringRepository creates a new scoring repository
func NewScoringRepository(db dbExecutor) ScoringRepository {
	return &scoringRepository{
		db:     db,
		engine: scoring.NewScoringEngine(),
	}
}

// GetActiveModels retrieves all active scoring models
func (r *scoringRepository) GetActiveModels() ([]scoring.ICPModel, error) {
	query := `
		SELECT id, name, description, rules, version, is_active, created_at, updated_at 
		FROM scoring_models 
		WHERE is_active = true
		ORDER BY name
	`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query scoring models: %w", err)
	}
	defer rows.Close()
	
	var models []scoring.ICPModel
	for rows.Next() {
		var id, name, description string
		var rulesJSON []byte
		var version int
		var isActive bool
		var createdAt, updatedAt time.Time
		
		err := rows.Scan(&id, &name, &description, &rulesJSON, &version, &isActive, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan scoring model: %w", err)
		}
		
		model, err := r.engine.LoadICPModelFromJSON(id, name, description, version, rulesJSON, isActive, createdAt, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to load ICP model %s from JSON: %w", name, err)
		}
		
		models = append(models, *model)
	}
	
	return models, nil
}

// GetModelByID retrieves a specific scoring model by ID
func (r *scoringRepository) GetModelByID(id string) (*scoring.ICPModel, error) {
	query := `
		SELECT id, name, description, rules, version, is_active, created_at, updated_at 
		FROM scoring_models 
		WHERE id = $1
	`
	
	var modelID, name, description string
	var rulesJSON []byte
	var version int
	var isActive bool
	var createdAt, updatedAt time.Time
	
	err := r.db.QueryRow(query, id).Scan(&modelID, &name, &description, &rulesJSON, &version, &isActive, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("scoring model %s not found", id)
		}
		return nil, fmt.Errorf("failed to get scoring model: %w", err)
	}
	
	return r.engine.LoadICPModelFromJSON(modelID, name, description, version, rulesJSON, isActive, createdAt, updatedAt)
}

// CreateModel creates a new scoring model
func (r *scoringRepository) CreateModel(model *scoring.ICPModel, userID uuid.UUID) error {
	// Convert model to JSON
	rules := map[string]interface{}{
		"must_have":      model.Requirements,
		"must_not":       model.Exclusions,
		"scoring_rules":  model.Rules,
		"minimum_score":  model.MinScore,
	}
	
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}
	
	// Generate ID if not provided
	if model.ID == "" {
		model.ID = uuid.New().String()
	}
	
	query := `
		INSERT INTO scoring_models (id, name, description, rules, version, is_active, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	
	now := time.Now()
	model.CreatedAt = now
	model.UpdatedAt = now
	
	_, err = r.db.Exec(query, model.ID, model.Name, model.Description, rulesJSON, model.Version, model.IsActive, userID, now, now)
	if err != nil {
		return fmt.Errorf("failed to create scoring model: %w", err)
	}
	
	return nil
}

// UpdateModel updates an existing scoring model
func (r *scoringRepository) UpdateModel(model *scoring.ICPModel) error {
	// Increment version
	model.Version++
	
	// Convert model to JSON
	rules := map[string]interface{}{
		"must_have":      model.Requirements,
		"must_not":       model.Exclusions,
		"scoring_rules":  model.Rules,
		"minimum_score":  model.MinScore,
	}
	
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}
	
	query := `
		UPDATE scoring_models 
		SET name = $2, description = $3, rules = $4, version = $5, is_active = $6, updated_at = $7
		WHERE id = $1
	`
	
	model.UpdatedAt = time.Now()
	result, err := r.db.Exec(query, model.ID, model.Name, model.Description, rulesJSON, model.Version, model.IsActive, model.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update scoring model: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("scoring model %s not found", model.ID)
	}
	
	return nil
}

// DeleteModel soft deletes a scoring model
func (r *scoringRepository) DeleteModel(id string) error {
	query := `
		UPDATE scoring_models 
		SET is_active = false, updated_at = $2
		WHERE id = $1
	`
	
	result, err := r.db.Exec(query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete scoring model: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("scoring model %s not found", id)
	}
	
	return nil
}

// StoreScore stores a scoring result
func (r *scoringRepository) StoreScore(score *scoring.ScoreResult) error {
	breakdownJSON, err := json.Marshal(score.Breakdown)
	if err != nil {
		return fmt.Errorf("failed to marshal score breakdown: %w", err)
	}
	
	// Parse company ID as UUID
	companyID, err := uuid.Parse(score.CompanyID)
	if err != nil {
		return fmt.Errorf("invalid company ID format: %w", err)
	}
	
	query := `
		INSERT INTO company_scores (company_id, scoring_model_id, score, qualified, requirements_met, score_breakdown, scored_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (company_id, scoring_model_id) 
		DO UPDATE SET 
			score = $3, 
			qualified = $4, 
			requirements_met = $5, 
			score_breakdown = $6, 
			scored_at = $7
	`
	
	_, err = r.db.Exec(query, companyID, score.ScoringModelID, score.Score, score.Qualified, score.RequirementsMet, breakdownJSON, score.ScoredAt)
	if err != nil {
		return fmt.Errorf("failed to store score result: %w", err)
	}
	
	return nil
}

// GetScoresByCompany retrieves all scores for a company
func (r *scoringRepository) GetScoresByCompany(companyID uuid.UUID) ([]scoring.ScoreResult, error) {
	query := `
		SELECT cs.scoring_model_id, cs.score, cs.qualified, cs.requirements_met, 
		       cs.score_breakdown, cs.scored_at, sm.name as model_name
		FROM company_scores cs
		JOIN scoring_models sm ON cs.scoring_model_id = sm.id
		WHERE cs.company_id = $1
		ORDER BY cs.scored_at DESC
	`
	
	rows, err := r.db.Query(query, companyID)
	if err != nil {
		return nil, fmt.Errorf("failed to query company scores: %w", err)
	}
	defer rows.Close()
	
	var scores []scoring.ScoreResult
	for rows.Next() {
		var modelID, modelName string
		var score int
		var qualified, requirementsMet bool
		var breakdownJSON []byte
		var scoredAt time.Time
		
		err := rows.Scan(&modelID, &score, &qualified, &requirementsMet, &breakdownJSON, &scoredAt, &modelName)
		if err != nil {
			return nil, fmt.Errorf("failed to scan score result: %w", err)
		}
		
		var breakdown map[string]scoring.ScoreDetail
		if err := json.Unmarshal(breakdownJSON, &breakdown); err != nil {
			return nil, fmt.Errorf("failed to unmarshal score breakdown: %w", err)
		}
		
		result := scoring.ScoreResult{
			CompanyID:       companyID.String(),
			ScoringModelID:  modelID,
			Score:           score,
			Qualified:       qualified,
			RequirementsMet: requirementsMet,
			Breakdown:       breakdown,
			ScoredAt:        scoredAt,
		}
		
		scores = append(scores, result)
	}
	
	return scores, nil
}

// GetScoresByModel retrieves all scores for a specific model
func (r *scoringRepository) GetScoresByModel(modelID string) ([]scoring.ScoreResult, error) {
	query := `
		SELECT cs.company_id, cs.score, cs.qualified, cs.requirements_met, 
		       cs.score_breakdown, cs.scored_at
		FROM company_scores cs
		WHERE cs.scoring_model_id = $1
		ORDER BY cs.scored_at DESC
	`
	
	rows, err := r.db.Query(query, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query model scores: %w", err)
	}
	defer rows.Close()
	
	var scores []scoring.ScoreResult
	for rows.Next() {
		var companyID uuid.UUID
		var score int
		var qualified, requirementsMet bool
		var breakdownJSON []byte
		var scoredAt time.Time
		
		err := rows.Scan(&companyID, &score, &qualified, &requirementsMet, &breakdownJSON, &scoredAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan score result: %w", err)
		}
		
		var breakdown map[string]scoring.ScoreDetail
		if err := json.Unmarshal(breakdownJSON, &breakdown); err != nil {
			return nil, fmt.Errorf("failed to unmarshal score breakdown: %w", err)
		}
		
		result := scoring.ScoreResult{
			CompanyID:       companyID.String(),
			ScoringModelID:  modelID,
			Score:           score,
			Qualified:       qualified,
			RequirementsMet: requirementsMet,
			Breakdown:       breakdown,
			ScoredAt:        scoredAt,
		}
		
		scores = append(scores, result)
	}
	
	return scores, nil
}

// DeleteScoresByCompany deletes all scores for a company
func (r *scoringRepository) DeleteScoresByCompany(companyID uuid.UUID) error {
	query := `DELETE FROM company_scores WHERE company_id = $1`
	
	_, err := r.db.Exec(query, companyID)
	if err != nil {
		return fmt.Errorf("failed to delete company scores: %w", err)
	}
	
	return nil
}

// DeleteScoresByModel deletes all scores for a model
func (r *scoringRepository) DeleteScoresByModel(modelID string) error {
	query := `DELETE FROM company_scores WHERE scoring_model_id = $1`
	
	_, err := r.db.Exec(query, modelID)
	if err != nil {
		return fmt.Errorf("failed to delete model scores: %w", err)
	}
	
	return nil
}