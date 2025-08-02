package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	DatabaseURL       string
	JWTSecret        string
	Port             string
	Environment      string
	DropContactAPIKey string
	OxyLabsUsername   string
	OxyLabsPassword   string
	OxyLabsEndpoint   string
	// Security configuration
	AllowedOrigins    string
	TrustedProxies    string
	EnableRateLimit   bool
	MaxRequestSize    int64
}

// New creates a new configuration instance from environment variables
func New() *Config {
	return &Config{
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		Port:             getEnv("PORT", "8080"),
		Environment:      getEnv("ENV", "development"),
		DropContactAPIKey: getEnv("DROPCONTACT_API_KEY", ""),
		OxyLabsUsername:   getEnv("OXYLABS_USERNAME", ""),
		OxyLabsPassword:   getEnv("OXYLABS_PASSWORD", ""),
		OxyLabsEndpoint:   getEnv("OXYLABS_ENDPOINT", "https://realtime.oxylabs.io/v1/queries"),
		// Security configuration
		AllowedOrigins:    getEnv("ALLOWED_ORIGINS", ""),
		TrustedProxies:    getEnv("TRUSTED_PROXIES", ""),
		EnableRateLimit:   getEnv("ENABLE_RATE_LIMIT", "true") == "true",
		MaxRequestSize:    getEnvAsInt64("MAX_REQUEST_SIZE", 10*1024*1024), // 10MB default
	}
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// HasOxyLabsCredentials returns true if OxyLabs credentials are configured
func (c *Config) HasOxyLabsCredentials() bool {
	return c.OxyLabsUsername != "" && c.OxyLabsPassword != ""
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetAllowedOrigins returns a slice of allowed CORS origins
func (c *Config) GetAllowedOrigins() []string {
	if c.AllowedOrigins == "" {
		// Default production origins - update these for your production domains
		return []string{
			"https://your-production-domain.com",
			"https://www.your-production-domain.com",
		}
	}
	return strings.Split(c.AllowedOrigins, ",")
}

// GetTrustedProxies returns a slice of trusted proxy IPs
func (c *Config) GetTrustedProxies() []string {
	if c.TrustedProxies == "" {
		return []string{} // No trusted proxies by default
	}
	return strings.Split(c.TrustedProxies, ",")
}

// IsSecurityEnabled returns true if security features should be enabled
func (c *Config) IsSecurityEnabled() bool {
	return c.IsProduction() || getEnv("ENABLE_SECURITY", "false") == "true"
}