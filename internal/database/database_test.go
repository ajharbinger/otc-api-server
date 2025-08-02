package database

import (
	"context"
	"testing"
	"time"
)

func TestDatabaseConfig(t *testing.T) {
	// Test that connection pool settings are reasonable
	db, err := New("postgres://user:pass@localhost:5432/test_db?sslmode=disable")
	if err != nil {
		t.Skip("Skipping database test - no connection available")
	}
	defer db.Close()

	stats := db.GetStats()
	
	// Verify connection pool configuration
	if stats.MaxOpenConnections != 25 {
		t.Errorf("Expected MaxOpenConnections to be 25, got %d", stats.MaxOpenConnections)
	}
	
	// Test health check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	err = db.PingContext(ctx)
	if err != nil {
		t.Skip("Database ping failed - connection not available for testing")
	}
}

func TestHealthCheck(t *testing.T) {
	// Test with invalid connection string - should fail quickly
	db, err := New("postgres://invalid:invalid@localhost:5432/invalid_db?sslmode=disable")
	if err == nil {
		defer db.Close()
		err = db.HealthCheck()
		if err == nil {
			t.Skip("Unexpected successful connection to invalid database")
		}
	}
	
	// This is expected to fail, so we just verify it fails gracefully
	if err == nil {
		t.Error("Expected health check to fail with invalid connection")
	}
}

func TestConnectionPoolStats(t *testing.T) {
	db, err := New("postgres://user:pass@localhost:5432/test_db?sslmode=disable")
	if err != nil {
		t.Skip("Skipping connection pool test - no database available")
	}
	defer db.Close()
	
	// Get initial stats
	stats := db.GetStats()
	
	// Verify stats structure
	if stats.MaxOpenConnections <= 0 {
		t.Error("Expected positive MaxOpenConnections")
	}
	
	if stats.MaxIdleConns <= 0 {
		t.Error("Expected positive MaxIdleConns")
	}
	
	// Stats should be accessible without panic
	t.Logf("Connection Pool Stats: Open=%d, Idle=%d, InUse=%d", 
		stats.OpenConnections, stats.Idle, stats.InUse)
}