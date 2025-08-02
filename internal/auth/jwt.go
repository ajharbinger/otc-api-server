package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Constants for context keys
const (
	UserIDKey = "user_id"
)

// Claims represents JWT claims
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// JWTService handles JWT token operations
type JWTService struct {
	secret []byte
}

// NewJWTService creates a new JWT service
func NewJWTService(secret string) *JWTService {
	return &JWTService{
		secret: []byte(secret),
	}
}

// GenerateToken generates a JWT token for a user
func (j *JWTService) GenerateToken(claims Claims) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour)
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return tokenString, expiresAt, nil
}

// GenerateRefreshToken generates a refresh token with longer expiration
func (j *JWTService) GenerateRefreshToken(claims Claims) (string, time.Time, error) {
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return tokenString, expiresAt, nil
}

// ValidateToken validates a JWT token and returns claims
func (j *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// ValidateRefreshToken validates a refresh token and returns claims
func (j *JWTService) ValidateRefreshToken(tokenString string) (*Claims, error) {
	// Same validation logic as regular token for now
	return j.ValidateToken(tokenString)
}

// GenerateJWT is a convenience function that creates a JWT service and generates a token
func GenerateJWT(userID uuid.UUID, role, secret string) (string, time.Time, error) {
	service := NewJWTService(secret)
	claims := Claims{
		UserID: userID,
		Email:  "",
		Role:   role,
	}
	return service.GenerateToken(claims)
}

// JWTMiddleware creates a middleware that validates JWT tokens from cookies
func JWTMiddleware(secret string) gin.HandlerFunc {
	service := NewJWTService(secret)
	return func(c *gin.Context) {
		// Try to get token from cookie first
		tokenString, err := c.Cookie("auth_token")
		if err != nil {
			// Fallback to Authorization header for backwards compatibility during transition
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}

			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
				c.Abort()
				return
			}
		}

		claims, err := service.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set(UserIDKey, claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Next()
	}
}

// CSRFMiddleware validates CSRF tokens for state-changing operations
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only check CSRF for POST, PUT, DELETE, PATCH requests
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		csrfCookie, err := c.Cookie("csrf_token")
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token required in cookie"})
			c.Abort()
			return
		}

		csrfHeader := c.GetHeader("X-CSRF-Token")
		if csrfHeader == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token required in X-CSRF-Token header"})
			c.Abort()
			return
		}

		if csrfCookie != csrfHeader {
			c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token mismatch"})
			c.Abort()
			return
		}

		c.Next()
	}
}