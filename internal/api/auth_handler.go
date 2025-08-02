package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/auth"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

// AuthHandler handles authentication operations
type AuthHandler struct {
	db  *sql.DB
	cfg *config.Config
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		db:  db,
		cfg: cfg,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required"`
	Role     string `json:"role" binding:"required,oneof=admin user"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	User      models.User `json:"user"`
	ExpiresAt time.Time   `json:"expires_at"`
	CSRFToken string      `json:"csrf_token"`
}

// generateCSRFToken generates a cryptographically secure CSRF token
func generateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// setSecureCookie sets a secure HTTP-only cookie
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
	secure := c.Request.Header.Get("X-Forwarded-Proto") == "https" || c.Request.TLS != nil
	c.SetCookie(
		name,
		value,
		maxAge,
		"/",
		"",
		secure,
		true, // HttpOnly
	)
}

// clearCookie clears a cookie by setting it to empty with past expiration
func clearCookie(c *gin.Context, name string) {
	secure := c.Request.Header.Get("X-Forwarded-Proto") == "https" || c.Request.TLS != nil
	c.SetCookie(
		name,
		"",
		-1,
		"/",
		"",
		secure,
		true, // HttpOnly
	)
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by email
	var user models.User
	query := `
		SELECT id, email, password_hash, name, role, created_at, updated_at 
		FROM users 
		WHERE email = $1
	`
	
	err := h.db.QueryRow(query, req.Email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Verify password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Generate JWT token
	token, expiresAt, err := auth.GenerateJWT(user.ID, user.Role, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Generate CSRF token
	csrfToken, err := generateCSRFToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CSRF token"})
		return
	}

	// Set secure HTTP-only cookies
	maxAge := int(time.Until(expiresAt).Seconds())
	setSecureCookie(c, "auth_token", token, maxAge)
	setSecureCookie(c, "csrf_token", csrfToken, maxAge)

	// Clear password hash from response
	user.PasswordHash = ""

	c.JSON(http.StatusOK, AuthResponse{
		User:      user,
		ExpiresAt: expiresAt,
		CSRFToken: csrfToken,
	})
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	err := h.db.QueryRow(checkQuery, req.Email).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user
	user := models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: passwordHash,
		Name:         req.Name,
		Role:         req.Role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Insert user into database
	insertQuery := `
		INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	
	_, err = h.db.Exec(insertQuery,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.Role,
		user.CreatedAt,
		user.UpdatedAt,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate JWT token
	token, expiresAt, err := auth.GenerateJWT(user.ID, user.Role, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Generate CSRF token
	csrfToken, err := generateCSRFToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CSRF token"})
		return
	}

	// Set secure HTTP-only cookies
	maxAge := int(time.Until(expiresAt).Seconds())
	setSecureCookie(c, "auth_token", token, maxAge)
	setSecureCookie(c, "csrf_token", csrfToken, maxAge)

	// Clear password hash from response
	user.PasswordHash = ""

	c.JSON(http.StatusCreated, AuthResponse{
		User:      user,
		ExpiresAt: expiresAt,
		CSRFToken: csrfToken,
	})
}

// Logout handles user logout by clearing cookies
func (h *AuthHandler) Logout(c *gin.Context) {
	clearCookie(c, "auth_token")
	clearCookie(c, "csrf_token")
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}