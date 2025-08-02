package repository

import (
	"database/sql"
	"fmt"
)

// transactionManager implements TransactionManager
type transactionManager struct {
	db *sql.DB
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(db *sql.DB) TransactionManager {
	return &transactionManager{db: db}
}

// WithTransaction executes a function within a database transaction
func (tm *transactionManager) WithTransaction(fn func(repos *Repositories) error) error {
	tx, err := tm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	// Create repositories with the transaction
	repos := &Repositories{
		Company: NewCompanyRepository(dbExecutor(tx)),
		Scoring: NewScoringRepository(dbExecutor(tx)),
		User:    NewUserRepository(dbExecutor(tx)),
		Tx:      tm, // Keep the same transaction manager
	}
	
	// Execute the function
	err = fn(repos)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %v, rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("transaction failed: %w", err)
	}
	
	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// dbExecutor is an interface that both *sql.DB and *sql.Tx implement
type dbExecutor interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// NewRepositories creates a new repository collection
func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Company: NewCompanyRepository(dbExecutor(db)),
		Scoring: NewScoringRepository(dbExecutor(db)),
		User:    NewUserRepository(dbExecutor(db)),
		Tx:      NewTransactionManager(db),
	}
}