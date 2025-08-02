package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

// SecurityHeadersMiddleware adds comprehensive security headers to all responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking attacks
		c.Header("X-Frame-Options", "DENY")
		
		// Prevent MIME-type confusion attacks
		c.Header("X-Content-Type-Options", "nosniff")
		
		// Enable XSS protection (legacy but still useful)
		c.Header("X-XSS-Protection", "1; mode=block")
		
		// Control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Content Security Policy for API endpoints
		csp := "default-src 'none'; " +
			"script-src 'none'; " +
			"style-src 'none'; " +
			"img-src 'none'; " +
			"connect-src 'self'; " +
			"font-src 'none'; " +
			"object-src 'none'; " +
			"media-src 'none'; " +
			"frame-src 'none'; " +
			"base-uri 'none'; " +
			"form-action 'none'"
		c.Header("Content-Security-Policy", csp)
		
		// Prevent caching of sensitive data
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		
		// Remove server header to avoid information disclosure
		c.Header("Server", "")
		
		c.Next()
	}
}

// CORSMiddleware handles Cross-Origin Resource Sharing with environment-based configuration
func CORSMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Determine allowed origins based on environment
		var allowedOrigins []string
		if cfg.IsDevelopment() {
			// Development: Allow localhost and common dev ports
			allowedOrigins = []string{
				"http://localhost:3000",
				"http://localhost:3001", 
				"http://localhost:8080",
				"http://127.0.0.1:3000",
				"http://127.0.0.1:3001",
				"http://127.0.0.1:8080",
			}
		} else {
			// Production: Only allow specific domains from config
			allowedOrigins = cfg.GetAllowedOrigins()
		}
		
		// Check if origin is allowed
		isAllowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				isAllowed = true
				break
			}
		}
		
		if isAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		
		// Set other CORS headers
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours
		
		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// InputValidationMiddleware provides basic input validation and sanitization
func InputValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set maximum request size (10MB)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10*1024*1024)
		
		// Validate Content-Type for POST/PUT requests
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			contentType := c.GetHeader("Content-Type")
			if contentType == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Content-Type header is required",
				})
				c.Abort()
				return
			}
			
			// Only allow specific content types
			allowedTypes := []string{
				"application/json",
				"multipart/form-data",
				"application/x-www-form-urlencoded",
			}
			
			isValidType := false
			for _, allowedType := range allowedTypes {
				if strings.HasPrefix(contentType, allowedType) {
					isValidType = true
					break
				}
			}
			
			if !isValidType {
				c.JSON(http.StatusUnsupportedMediaType, gin.H{
					"error": "Unsupported content type",
					"allowed_types": allowedTypes,
				})
				c.Abort()
				return
			}
		}
		
		// Basic header validation
		userAgent := c.GetHeader("User-Agent")
		if userAgent == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "User-Agent header is required",
			})
			c.Abort()
			return
		}
		
		// Block potentially malicious user agents
		suspiciousPatterns := []string{
			"sqlmap",
			"nikto",
			"nmap",
			"masscan",
			"<script",
			"javascript:",
		}
		
		userAgentLower := strings.ToLower(userAgent)
		for _, pattern := range suspiciousPatterns {
			if strings.Contains(userAgentLower, pattern) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Request blocked for security reasons",
				})
				c.Abort()
				return
			}
		}
		
		c.Next()
	}
}

// RateLimitingMiddleware provides basic rate limiting
func RateLimitingMiddleware() gin.HandlerFunc {
	// Simple in-memory rate limiting (for production, use Redis)
	clients := make(map[string][]time.Time)
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()
		
		// Clean old entries (older than 1 minute)
		if timestamps, exists := clients[clientIP]; exists {
			var validTimestamps []time.Time
			for _, timestamp := range timestamps {
				if now.Sub(timestamp) <= time.Minute {
					validTimestamps = append(validTimestamps, timestamp)
				}
			}
			clients[clientIP] = validTimestamps
		}
		
		// Check rate limit (100 requests per minute per IP)
		if len(clients[clientIP]) >= 100 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": "60",
			})
			c.Header("Retry-After", "60")
			c.Abort()
			return
		}
		
		// Add current timestamp
		clients[clientIP] = append(clients[clientIP], now)
		
		c.Next()
	}
}

// LoggingMiddleware provides security-focused request logging
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		
		// Process request
		c.Next()
		
		// Log request details
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		
		if raw != "" {
			path = path + "?" + raw
		}
		
		// Log security-relevant information
		fmt.Printf("[SECURITY] %v | %3d | %13v | %15s | %-7s %s | UA: %s\n",
			start.Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
			c.Request.UserAgent(),
		)
		
		// Log suspicious activity
		if statusCode >= 400 {
			fmt.Printf("[SECURITY-ALERT] %d response for %s %s from %s | UA: %s\n",
				statusCode, method, path, clientIP, c.Request.UserAgent())
		}
	}
}