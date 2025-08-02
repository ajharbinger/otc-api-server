package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/repository"
	"github.com/ajharbinger/otc-oxy2-pipeline/internal/services"
)

// AuthHandlerV2 handles authentication operations with the new service layer
type AuthHandlerV2 struct {
	authService services.AuthService
}

// NewAuthHandlerV2 creates a new auth handler with service injection
func NewAuthHandlerV2(authService services.AuthService) *AuthHandlerV2 {
	return &AuthHandlerV2{
		authService: authService,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates a user
func (h *AuthHandlerV2) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	response, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Register creates a new user account
func (h *AuthHandlerV2) Register(c *gin.Context) {
	var req repository.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	user, err := h.authService.Register(&req)
	if err != nil {
		if err.Error() == "user with email "+req.Email+" already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user":    user,
	})
}

// RefreshToken generates a new access token from a refresh token
func (h *AuthHandlerV2) RefreshToken(c *gin.Context) {
	type RefreshRequest struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	response, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, response)
}