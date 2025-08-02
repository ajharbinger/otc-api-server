package scraper

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Client handles HTTP requests to OTC Markets with rate limiting
type Client struct {
	httpClient  *http.Client
	rateLimiter chan struct{}
	userAgent   string
}

// NewClient creates a new scraping client with rate limiting
func NewClient(requestsPerSecond int) *Client {
	// Create rate limiter channel
	rateLimiter := make(chan struct{}, requestsPerSecond)
	
	// Fill the rate limiter initially
	for i := 0; i < requestsPerSecond; i++ {
		rateLimiter <- struct{}{}
	}
	
	// Start refilling the rate limiter
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
		defer ticker.Stop()
		
		for range ticker.C {
			select {
			case rateLimiter <- struct{}{}:
			default:
			}
		}
	}()

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		rateLimiter: rateLimiter,
		userAgent:   "Mozilla/5.0 (compatible; OTC-Scraper/1.0)",
	}
}

// Get performs a rate-limited HTTP GET request and returns a goquery document
func (c *Client) Get(ctx context.Context, url string) (*goquery.Document, error) {
	// Wait for rate limiter
	select {
	case <-c.rateLimiter:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc, nil
}

// Close cleans up the client resources
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}