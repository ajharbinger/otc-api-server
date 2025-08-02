package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		expectedHeaders map[string]string
	}{
		{
			name: "Security headers are set correctly",
			expectedHeaders: map[string]string{
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff", 
				"X-XSS-Protection":          "1; mode=block",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Content-Security-Policy":   "default-src 'none'",
				"Cache-Control":             "no-store, no-cache, must-revalidate, proxy-revalidate",
				"Pragma":                    "no-cache",
				"Expires":                   "0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(SecurityHeadersMiddleware())
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			for header, expectedValue := range tt.expectedHeaders {
				actualValue := w.Header().Get(header)
				if header == "Content-Security-Policy" {
					// Check if CSP contains expected directive
					assert.Contains(t, actualValue, expectedValue, "CSP should contain %s", expectedValue)
				} else {
					assert.Equal(t, expectedValue, actualValue, "Header %s should be %s", header, expectedValue)
				}
			}
		})
	}
}

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		environment    string
		origin         string
		expectedOrigin string
		shouldAllow    bool
	}{
		{
			name:           "Development - localhost allowed",
			environment:    "development",
			origin:         "http://localhost:3000",
			expectedOrigin: "http://localhost:3000",
			shouldAllow:    true,
		},
		{
			name:           "Development - localhost 8080 allowed",
			environment:    "development", 
			origin:         "http://localhost:8080",
			expectedOrigin: "http://localhost:8080",
			shouldAllow:    true,
		},
		{
			name:        "Development - unknown origin blocked",
			environment: "development",
			origin:      "https://malicious-site.com",
			shouldAllow: false,
		},
		{
			name:        "Production - unknown origin blocked",
			environment: "production",
			origin:      "https://malicious-site.com", 
			shouldAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Environment: tt.environment,
			}

			router := gin.New()
			router.Use(CORSMiddleware(cfg))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if tt.shouldAllow {
				assert.Equal(t, tt.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}

			// Check other CORS headers are always set
			assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
			assert.Equal(t, "Origin, Content-Type, Accept, Authorization, X-Requested-With", w.Header().Get("Access-Control-Allow-Headers"))
			assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &config.Config{
		Environment: "development",
	}

	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestInputValidationMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		method         string
		contentType    string
		userAgent      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Valid POST request",
			method:         "POST",
			contentType:    "application/json",
			userAgent:      "Mozilla/5.0",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST without Content-Type",
			method:         "POST",
			contentType:    "",
			userAgent:      "Mozilla/5.0",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Content-Type header is required",
		},
		{
			name:           "POST with invalid Content-Type",
			method:         "POST",
			contentType:    "text/html",
			userAgent:      "Mozilla/5.0",
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "Unsupported content type",
		},
		{
			name:           "Request without User-Agent",
			method:         "GET",
			contentType:    "",
			userAgent:      "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "User-Agent header is required",
		},
		{
			name:           "Suspicious User-Agent (sqlmap)",
			method:         "GET",
			contentType:    "",
			userAgent:      "sqlmap/1.4.9",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Request blocked for security reasons",
		},
		{
			name:           "Suspicious User-Agent (nikto)",
			method:         "GET",
			contentType:    "",
			userAgent:      "Nikto/2.1.6",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Request blocked for security reasons",
		},
		{
			name:           "Suspicious User-Agent (script tag)",
			method:         "GET",
			contentType:    "",
			userAgent:      "Mozilla <script>alert('xss')</script>",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Request blocked for security reasons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(InputValidationMiddleware())
			router.Any("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestRateLimitingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RateLimitingMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Make multiple requests from the same IP
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345" // Simulate same IP
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i < 9 {
			assert.Equal(t, http.StatusOK, w.Code)
		}
	}
}

func TestLoggingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LoggingMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Note: Testing logging output would require capturing stdout,
	// which is complex and not essential for this test
}