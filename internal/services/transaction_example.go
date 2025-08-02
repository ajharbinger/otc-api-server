package services

import (
	"fmt"

	"github.com/ajharbinger/otc-oxy2-pipeline/internal/errors"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
)

// Example of complex operation requiring transaction support
// This would be used for operations that need to update multiple tables atomically

// BulkScoreCompaniesWithTransaction scores multiple companies within a transaction
func (s *scoringServiceImpl) BulkScoreCompaniesWithTransaction(companyIDs []string, modelID string) error {
	s.logger.Info("Starting bulk scoring operation with transaction", "company_count", len(companyIDs), "model_id", modelID)

	// Use transaction manager to ensure atomicity
	return s.repos.Tx.WithTransaction(func(repos *repository.Repositories) error {
		// Validate model exists
		model, err := repos.Scoring.GetModelByID(modelID)
		if err != nil {
			s.logger.Error("Model not found for bulk scoring", err, "model_id", modelID)
			return errors.NotFound("scoring model not found", err).WithOperation("BulkScoreCompaniesWithTransaction")
		}

		s.logger.Info("Using model for bulk scoring", "model_name", model.Name, "model_version", model.Version)

		// Score each company within the transaction
		successCount := 0
		for i, companyID := range companyIDs {
			s.logger.Debug("Scoring company", "company_id", companyID, "progress", fmt.Sprintf("%d/%d", i+1, len(companyIDs)))

			// Get company data
			companyData, err := s.getCompanyDataFromRepos(companyID, repos)
			if err != nil {
				s.logger.Warn("Failed to get company data, skipping", "company_id", companyID, "error", err)
				continue
			}

			// Score company
			result, err := s.engine.ScoreCompany(companyData, *model)
			if err != nil {
				s.logger.Warn("Failed to score company, skipping", "company_id", companyID, "error", err)
				continue
			}

			result.CompanyID = companyID

			// Store result within transaction
			if err := repos.Scoring.StoreScore(result); err != nil {
				s.logger.Error("Failed to store score result", err, "company_id", companyID)
				return errors.DatabaseError("failed to store score result", err).WithOperation("BulkScoreCompaniesWithTransaction")
			}

			successCount++
		}

		s.logger.Info("Bulk scoring completed successfully", "total_companies", len(companyIDs), "successful_scores", successCount)

		// If we didn't score any companies successfully, return an error to rollback
		if successCount == 0 {
			return errors.ServiceError("no companies were scored successfully", nil).WithOperation("BulkScoreCompaniesWithTransaction")
		}

		return nil
	})
}

// getCompanyDataFromRepos is a helper to get company data within a transaction context
func (s *scoringServiceImpl) getCompanyDataFromRepos(companyID string, repos *repository.Repositories) (map[string]interface{}, error) {
	// Implementation would use repos.Company.GetByID and convert to map
	// This is similar to the existing getCompanyData but uses transaction-aware repos
	return map[string]interface{}{}, nil // Placeholder implementation
}

// Example service method that demonstrates transaction rollback on error
func (s *scoringServiceImpl) CreateModelAndScore(form *repository.ScoringModelForm, userID string, testCompanyID string) (*repository.ScoringModel, *repository.CompanyScore, error) {
	s.logger.Info("Creating new scoring model and testing on company", "user_id", userID, "test_company", testCompanyID)

	var createdModel *repository.ScoringModel
	var testScore *repository.CompanyScore

	err := s.repos.Tx.WithTransaction(func(repos *repository.Repositories) error {
		// Create the model
		model, err := s.createModelInTransaction(form, userID, repos)
		if err != nil {
			return err
		}
		createdModel = model

		// Test score on a company
		score, err := s.scoreCompanyInTransaction(testCompanyID, model.ID, repos)
		if err != nil {
			s.logger.Error("Failed to test score new model", err, "model_id", model.ID, "company_id", testCompanyID)
			return errors.ServiceError("failed to test new scoring model", err).WithOperation("CreateModelAndScore")
		}
		testScore = score

		s.logger.Info("Successfully created and tested new scoring model", "model_id", model.ID, "test_score", score.Score)
		return nil
	})

	if err != nil {
		s.logger.Error("Transaction failed for model creation and testing", err)
		return nil, nil, err
	}

	return createdModel, testScore, nil
}

// Helper methods for transaction operations
func (s *scoringServiceImpl) createModelInTransaction(form *repository.ScoringModelForm, userID string, repos *repository.Repositories) (*repository.ScoringModel, error) {
	// Implementation would use repos.Scoring.CreateModel
	return nil, nil // Placeholder
}

func (s *scoringServiceImpl) scoreCompanyInTransaction(companyID, modelID string, repos *repository.Repositories) (*repository.CompanyScore, error) {
	// Implementation would use repos to score and store
	return nil, nil // Placeholder
}