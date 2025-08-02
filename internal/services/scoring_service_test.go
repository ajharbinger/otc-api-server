package services

import (
	"testing"

	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/scoring"
)

// MockScoringRepository implements ScoringRepository for testing
type MockScoringRepository struct {
	models []scoring.ICPModel
	scores map[string][]scoring.ScoreResult
}

func NewMockScoringRepository() *MockScoringRepository {
	return &MockScoringRepository{
		models: []scoring.ICPModel{},
		scores: make(map[string][]scoring.ScoreResult),
	}
}

func (m *MockScoringRepository) GetActiveModels() ([]scoring.ICPModel, error) {
	var activeModels []scoring.ICPModel
	for _, model := range m.models {
		if model.IsActive {
			activeModels = append(activeModels, model)
		}
	}
	return activeModels, nil
}

func (m *MockScoringRepository) GetModelByID(id string) (*scoring.ICPModel, error) {
	for _, model := range m.models {
		if model.ID == id {
			return &model, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *MockScoringRepository) CreateModel(model *scoring.ICPModel, userID uuid.UUID) error {
	m.models = append(m.models, *model)
	return nil
}

func (m *MockScoringRepository) UpdateModel(model *scoring.ICPModel) error {
	for i, existing := range m.models {
		if existing.ID == model.ID {
			m.models[i] = *model
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *MockScoringRepository) DeleteModel(id string) error {
	for i, model := range m.models {
		if model.ID == id {
			m.models[i].IsActive = false
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *MockScoringRepository) StoreScore(score *scoring.ScoreResult) error {
	companyScores := m.scores[score.CompanyID]
	
	// Update or append score
	found := false
	for i, existing := range companyScores {
		if existing.ScoringModelID == score.ScoringModelID {
			companyScores[i] = *score
			found = true
			break
		}
	}
	
	if !found {
		companyScores = append(companyScores, *score)
	}
	
	m.scores[score.CompanyID] = companyScores
	return nil
}

func (m *MockScoringRepository) GetScoresByCompany(companyID uuid.UUID) ([]scoring.ScoreResult, error) {
	return m.scores[companyID.String()], nil
}

func (m *MockScoringRepository) GetScoresByModel(modelID string) ([]scoring.ScoreResult, error) {
	var results []scoring.ScoreResult
	for _, companyScores := range m.scores {
		for _, score := range companyScores {
			if score.ScoringModelID == modelID {
				results = append(results, score)
			}
		}
	}
	return results, nil
}

func (m *MockScoringRepository) DeleteScoresByCompany(companyID uuid.UUID) error {
	delete(m.scores, companyID.String())
	return nil
}

func (m *MockScoringRepository) DeleteScoresByModel(modelID string) error {
	for companyID, companyScores := range m.scores {
		var filtered []scoring.ScoreResult
		for _, score := range companyScores {
			if score.ScoringModelID != modelID {
				filtered = append(filtered, score)
			}
		}
		m.scores[companyID] = filtered
	}
	return nil
}

// Test example
func TestScoringService_GetActiveScoringModels(t *testing.T) {
	// Setup mocks
	mockScoringRepo := NewMockScoringRepository()
	mockCompanyRepo := &MockCompanyRepository{} // Would need to implement
	mockUserRepo := &MockUserRepository{}       // Would need to implement
	mockTxManager := &MockTransactionManager{}  // Would need to implement

	repos := &repository.Repositories{
		Scoring: mockScoringRepo,
		Company: mockCompanyRepo,
		User:    mockUserRepo,
		Tx:      mockTxManager,
	}

	// Add test data
	testModel := scoring.ICPModel{
		ID:       "test-model-1",
		Name:     "Test Model",
		IsActive: true,
	}
	mockScoringRepo.models = append(mockScoringRepo.models, testModel)

	// Create service
	service := newScoringService(repos)

	// Test
	models, err := service.GetActiveScoringModels()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(models) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(models))
	}

	if models[0].ID != "test-model-1" {
		t.Errorf("Expected model ID 'test-model-1', got '%s'", models[0].ID)
	}
}

// Additional test cases would include:
// - Error cases (database errors, validation errors)
// - Transaction rollback scenarios
// - Service integration tests
// - Performance benchmarks

// Example benchmark test
func BenchmarkScoringService_GetActiveScoringModels(b *testing.B) {
	// Setup similar to test above
	mockScoringRepo := NewMockScoringRepository()
	
	// Add multiple models for more realistic benchmark
	for i := 0; i < 100; i++ {
		model := scoring.ICPModel{
			ID:       fmt.Sprintf("model-%d", i),
			Name:     fmt.Sprintf("Model %d", i),
			IsActive: true,
		}
		mockScoringRepo.models = append(mockScoringRepo.models, model)
	}

	repos := &repository.Repositories{
		Scoring: mockScoringRepo,
		// ... other mocked repos
	}

	service := newScoringService(repos)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetActiveScoringModels()
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}