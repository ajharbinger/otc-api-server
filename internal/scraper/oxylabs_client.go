package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ajharbinger/otc-oxy2-pipeline/pkg/config"
)

// OxyLabsClient handles requests through OxyLabs Web Scraper API
type OxyLabsClient struct {
	httpClient *http.Client
	username   string
	password   string
	endpoint   string
}

// OxyLabsRequest represents a request to the OxyLabs API
type OxyLabsRequest struct {
	Source      string            `json:"source"`
	URL         string            `json:"url"`
	UserAgent   string            `json:"user_agent,omitempty"`
	Render      string            `json:"render,omitempty"`
	Context     []ContextParam    `json:"context,omitempty"`
	ParseInstr  map[string]interface{} `json:"parse,omitempty"`
}

// ContextParam represents context parameters for OxyLabs
type ContextParam struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// OxyLabsResponse represents the response from OxyLabs API
type OxyLabsResponse struct {
	Results []OxyLabsResult `json:"results"`
}

// OxyLabsResult represents a single result from OxyLabs
type OxyLabsResult struct {
	Content   string            `json:"content"`
	StatusCode int              `json:"status_code"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	TaskID    string            `json:"task_id,omitempty"`
}

// NewOxyLabsClient creates a new OxyLabs client
func NewOxyLabsClient(cfg *config.Config) *OxyLabsClient {
	return &OxyLabsClient{
		httpClient: &http.Client{
			Timeout: 180 * time.Second, // Increased timeout for rendered pages
		},
		username: cfg.OxyLabsUsername,
		password: cfg.OxyLabsPassword,
		endpoint: cfg.OxyLabsEndpoint,
	}
}

// Get performs a scraping request through OxyLabs and returns a goquery document
func (c *OxyLabsClient) Get(ctx context.Context, url string) (*goquery.Document, error) {
	// Prepare OxyLabs request with HTML rendering for OTC Markets
	oxyRequest := OxyLabsRequest{
		Source:    "universal",
		URL:       url,
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		Render:    "html", // Enable HTML rendering for JavaScript content
		Context: []ContextParam{
			{Key: "follow_redirects", Value: true},
			{Key: "return_only_content", Value: true},
		},
	}

	// Marshal request to JSON
	requestBody, err := json.Marshal(oxyRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.username, c.password)

	// Perform request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse OxyLabs response
	var oxyResponse OxyLabsResponse
	if err := json.Unmarshal(respBody, &oxyResponse); err != nil {
		return nil, fmt.Errorf("failed to parse OxyLabs response: %w", err)
	}

	// Check if we got results
	if len(oxyResponse.Results) == 0 {
		return nil, fmt.Errorf("no results returned from OxyLabs")
	}

	result := oxyResponse.Results[0]

	// Check the scraped page status code
	if result.StatusCode != 200 {
		return nil, fmt.Errorf("target page returned status code: %d", result.StatusCode)
	}

	// Parse HTML content with goquery
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(result.Content)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML content: %w", err)
	}

	return doc, nil
}

// GetBatch performs multiple scraping requests concurrently through OxyLabs
func (c *OxyLabsClient) GetBatch(ctx context.Context, urls []string) (map[string]*goquery.Document, map[string]error) {
	docs := make(map[string]*goquery.Document)
	errors := make(map[string]error)

	// Prepare batch request with HTML rendering
	requests := make([]OxyLabsRequest, len(urls))
	for i, url := range urls {
		requests[i] = OxyLabsRequest{
			Source:    "universal",
			URL:       url,
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			Render:    "html", // Enable HTML rendering for JavaScript content
			Context: []ContextParam{
				{Key: "follow_redirects", Value: true},
				{Key: "return_only_content", Value: true},
			},
		}
	}

	// Marshal batch request
	requestBody, err := json.Marshal(requests)
	if err != nil {
		// If batch request fails, fall back to individual requests
		for _, url := range urls {
			doc, err := c.Get(ctx, url)
			if err != nil {
				errors[url] = err
			} else {
				docs[url] = doc
			}
		}
		return docs, errors
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		// Fall back to individual requests
		for _, url := range urls {
			doc, err := c.Get(ctx, url)
			if err != nil {
				errors[url] = err
			} else {
				docs[url] = doc
			}
		}
		return docs, errors
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.username, c.password)

	// Perform request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Fall back to individual requests
		for _, url := range urls {
			doc, err := c.Get(ctx, url)
			if err != nil {
				errors[url] = err
			} else {
				docs[url] = doc
			}
		}
		return docs, errors
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// Fall back to individual requests
		for _, url := range urls {
			doc, err := c.Get(ctx, url)
			if err != nil {
				errors[url] = err
			} else {
				docs[url] = doc
			}
		}
		return docs, errors
	}

	// Parse batch response
	var oxyResponse OxyLabsResponse
	if err := json.Unmarshal(respBody, &oxyResponse); err != nil {
		// Fall back to individual requests
		for _, url := range urls {
			doc, err := c.Get(ctx, url)
			if err != nil {
				errors[url] = err
			} else {
				docs[url] = doc
			}
		}
		return docs, errors
	}

	// Process batch results
	for i, result := range oxyResponse.Results {
		if i >= len(urls) {
			break
		}
		
		url := urls[i]
		
		if result.StatusCode != 200 {
			errors[url] = fmt.Errorf("target page returned status code: %d", result.StatusCode)
			continue
		}

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(result.Content)))
		if err != nil {
			errors[url] = fmt.Errorf("failed to parse HTML content: %w", err)
		} else {
			docs[url] = doc
		}
	}

	return docs, errors
}

// Health checks if the OxyLabs API is accessible
func (c *OxyLabsClient) Health(ctx context.Context) error {
	// Test with a simple request
	testURL := "https://httpbin.org/status/200"
	
	_, err := c.Get(ctx, testURL)
	if err != nil {
		return fmt.Errorf("OxyLabs health check failed: %w", err)
	}
	
	return nil
}

// Close cleans up client resources
func (c *OxyLabsClient) Close() {
	c.httpClient.CloseIdleConnections()
}