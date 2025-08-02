package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
)

// userRepository implements UserRepository
type userRepository struct {
	db dbExecutor
}

// NewUserRepository creates a new user repository
func NewUserRepository(db dbExecutor) UserRepository {
	return &userRepository{db: db}
}

// GetByID retrieves a user by ID
func (r *userRepository) GetByID(id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, role, created_at, updated_at
		FROM users WHERE id = $1
	`
	
	user := &models.User{}
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Role,
		&user.CreatedAt, &user.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return user, nil
}

// GetByEmail retrieves a user by email
func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, role, created_at, updated_at
		FROM users WHERE email = $1
	`
	
	user := &models.User{}
	err := r.db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Role,
		&user.CreatedAt, &user.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with email %s not found", email)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return user, nil
}

// Create creates a new user
func (r *userRepository) Create(user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	
	query := `
		INSERT INTO users (id, email, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	
	_, err := r.db.Exec(query,
		user.ID, user.Email, user.PasswordHash, user.Role,
		user.CreatedAt, user.UpdatedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	
	return nil
}

// Update updates an existing user
func (r *userRepository) Update(user *models.User) error {
	user.UpdatedAt = time.Now()
	
	query := `
		UPDATE users SET
			email = $2, password_hash = $3, role = $4, updated_at = $5
		WHERE id = $1
	`
	
	result, err := r.db.Exec(query,
		user.ID, user.Email, user.PasswordHash, user.Role, user.UpdatedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	
	return nil
}

// Delete deletes a user
func (r *userRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	
	return nil
}