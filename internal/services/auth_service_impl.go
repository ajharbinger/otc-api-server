package services

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"github.com/google/uuid"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/auth"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

// authServiceImpl implements AuthService
type authServiceImpl struct {
	repos      *repository.Repositories
	jwtService *auth.JWTService
	cfg        *config.Config
}

// newAuthService creates a new auth service implementation
func newAuthService(repos *repository.Repositories, cfg *config.Config) AuthService {
	return &authServiceImpl{
		repos:      repos,
		jwtService: auth.NewJWTService(cfg.JWTSecret),
		cfg:        cfg,
	}
}

// Login authenticates a user and returns a token
func (s *authServiceImpl) Login(email, password string) (*repository.LoginResponse, error) {
	// Get user by email
	user, err := s.repos.User.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate JWT token
	claims := auth.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}

	token, expiresAt, err := s.jwtService.GenerateToken(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Generate refresh token (simplified - in production, store in database)
	refreshToken, _, err := s.jwtService.GenerateRefreshToken(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &repository.LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User: models.User{
			ID:        user.ID,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		ExpiresAt: expiresAt,
	}, nil
}

// Register creates a new user account
func (s *authServiceImpl) Register(req *repository.RegisterRequest) (*models.User, error) {
	// Check if user already exists
	existingUser, err := s.repos.User.GetByEmail(req.Email)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Set default role if not provided
	role := req.Role
	if role == "" {
		role = "user"
	}

	// Validate role
	if role != "user" && role != "admin" {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	// Create user
	user := &models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Role:         role,
	}

	if err := s.repos.User.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Clear password hash from response
	user.PasswordHash = ""

	return user, nil
}

// ValidateToken validates a JWT token and returns the user
func (s *authServiceImpl) ValidateToken(token string) (*models.User, error) {
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Get user from database to ensure they still exist
	user, err := s.repos.User.GetByID(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &models.User{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

// RefreshToken generates a new token from a refresh token
func (s *authServiceImpl) RefreshToken(refreshToken string) (*repository.LoginResponse, error) {
	claims, err := s.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Get user from database
	user, err := s.repos.User.GetByID(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Generate new tokens
	newClaims := auth.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}

	token, expiresAt, err := s.jwtService.GenerateToken(newClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	newRefreshToken, _, err := s.jwtService.GenerateRefreshToken(newClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &repository.LoginResponse{
		Token:        token,
		RefreshToken: newRefreshToken,
		User: models.User{
			ID:        user.ID,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		ExpiresAt: expiresAt,
	}, nil
}