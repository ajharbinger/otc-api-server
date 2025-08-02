package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	handler := &UploadHandler{}

	testCases := []struct {
		name        string
		csvContent  string
		expected    []string
		expectError bool
	}{
		{
			name:       "Valid CSV with single ticker",
			csvContent: "AAPL\n",
			expected:   []string{"AAPL"},
		},
		{
			name:       "Valid CSV with multiple tickers",
			csvContent: "AAPL\nMSFT\nGOOGL\n",
			expected:   []string{"AAPL", "MSFT", "GOOGL"},
		},
		{
			name:       "CSV with header",
			csvContent: "ticker\nAAPL\nMSFT\n",
			expected:   []string{"AAPL", "MSFT"},
		},
		{
			name:       "CSV with duplicates",
			csvContent: "AAPL\nAAPL\nMSFT\n",
			expected:   []string{"AAPL", "MSFT"},
		},
		{
			name:       "CSV with whitespace",
			csvContent: " AAPL \n  MSFT  \n",
			expected:   []string{"AAPL", "MSFT"},
		},
		{
			name:        "Empty CSV",
			csvContent:  "",
			expectError: true,
		},
		{
			name:        "Invalid ticker format",
			csvContent:  "INVALID_TICKER_TOO_LONG\n",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.csvContent)
			tickers, err := handler.parseCSV(reader)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(tickers) != len(tc.expected) {
				t.Errorf("Expected %d tickers, got %d", len(tc.expected), len(tickers))
				return
			}

			for i, expected := range tc.expected {
				if tickers[i] != expected {
					t.Errorf("Expected ticker %s, got %s", expected, tickers[i])
				}
			}
		})
	}
}

func TestIsValidTicker(t *testing.T) {
	handler := &UploadHandler{}

	testCases := []struct {
		ticker string
		valid  bool
	}{
		{"AAPL", true},
		{"MSFT", true},
		{"A", true},
		{"ABCDEFGHIJ", true}, // 10 chars - max allowed
		{"", false},
		{"ABCDEFGHIJK", false}, // 11 chars - too long
		{"AAPL!", false},       // Invalid character
		{"AA PL", false},       // Space not allowed
		{"123", true},          // Numbers allowed
		{"ABC123", true},       // Alphanumeric allowed
	}

	for _, tc := range testCases {
		t.Run(tc.ticker, func(t *testing.T) {
			result := handler.isValidTicker(tc.ticker)
			if result != tc.valid {
				t.Errorf("Expected isValidTicker(%s) = %v, got %v", tc.ticker, tc.valid, result)
			}
		})
	}
}

func createTestCSV(content string) (io.Reader, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	fileWriter, err := writer.CreateFormFile("csv_file", "test.csv")
	if err != nil {
		return nil, "", err
	}
	
	_, err = fileWriter.Write([]byte(content))
	if err != nil {
		return nil, "", err
	}
	
	err = writer.Close()
	if err != nil {
		return nil, "", err
	}
	
	return &buf, writer.FormDataContentType(), nil
}