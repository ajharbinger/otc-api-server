# CSV Upload API Documentation

This document describes the CSV upload API endpoints for the OTC Oxy Scraper platform.

## Authentication

All protected endpoints require a JWT token in the Authorization header:
```
Authorization: Bearer <your-jwt-token>
```

## Endpoints

### POST /api/v1/auth/login
Login with email and password.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "your-password"
}
```

**Response:**
```json
{
  "token": "jwt-token-here",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "User Name",
    "role": "admin|user"
  },
  "expires_at": "2025-07-31T10:00:00Z"
}
```

### POST /api/v1/auth/register
Register a new user account.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "your-password",
  "name": "User Name", 
  "role": "admin|user"
}
```

### POST /api/v1/upload/csv
Upload a CSV file containing ticker symbols for scraping.

**Content-Type:** `multipart/form-data`

**Form Fields:**
- `csv_file` (file): CSV file containing ticker symbols
- `use_optimized` (boolean, optional): Whether to use optimized batch processing for large files

**CSV Format:**
- First column should contain ticker symbols
- Headers are optional (common headers like "ticker", "symbol" are automatically ignored)
- Ticker symbols will be converted to uppercase
- Duplicates are automatically removed
- Maximum 10,000 tickers per upload

**Example CSV:**
```
ticker
AAPL
MSFT
GOOGL
```

**Response:**
```json
{
  "message": "CSV upload successful, scraping job started",
  "job_id": "uuid-of-scraping-job",
  "total_tickers": 3,
  "status": "pending"
}
```

**Error Responses:**
- `400 Bad Request`: Invalid CSV format, too many tickers, or invalid file
- `401 Unauthorized`: Missing or invalid JWT token
- `500 Internal Server Error`: Server error during processing

### GET /api/v1/jobs
Retrieve all scraping jobs for the authenticated user.

**Response:**
```json
{
  "jobs": [
    {
      "id": "uuid",
      "status": "pending|running|completed|failed",
      "total_tickers": 100,
      "processed_tickers": 50,
      "failed_tickers": 2,
      "started_by": "user-uuid",
      "started_at": "2025-07-30T10:00:00Z",
      "completed_at": "2025-07-30T11:00:00Z",
      "error_message": ""
    }
  ]
}
```

### GET /api/v1/jobs/:id
Retrieve a specific scraping job by ID.

**Response:**
```json
{
  "job": {
    "id": "uuid",
    "status": "completed",
    "total_tickers": 100,
    "processed_tickers": 98,
    "failed_tickers": 2,
    "started_by": "user-uuid",
    "started_at": "2025-07-30T10:00:00Z",
    "completed_at": "2025-07-30T11:00:00Z",
    "error_message": ""
  }
}
```

### GET /api/v1/companies
Retrieve paginated company data with filtering and search capabilities.

**Query Parameters:**
- `page` (int, optional): Page number (default: 1)
- `limit` (int, optional): Results per page (default: 50, max: 1000)
- `search` (string, optional): Search term for company name/ticker (case-insensitive)
- `market_tier` (string, optional): Filter by market tier

**Response:**
```json
{
  "companies": [
    {
      "id": "uuid",
      "ticker": "AAPL",
      "company_name": "Apple Inc.",
      "market_tier": "OTCQX",
      "quote_status": "Active",
      "trading_volume": 1000000,
      "website": "https://apple.com",
      "description": "Technology company...",
      "officers": [...],
      "address": {...},
      "transfer_agent": "...",
      "auditor": "...",
      "last_10k_date": "2024-10-30T00:00:00Z",
      "last_10q_date": "2024-07-30T00:00:00Z",
      "last_filing_date": "2024-10-30T00:00:00Z",
      "profile_verified": true,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-10-30T00:00:00Z"
    }
  ],
  "page": 1,
  "limit": 50,
  "total": 150,
  "total_pages": 3
}
```

### GET /api/v1/companies/:ticker
Retrieve a specific company by ticker symbol.

**Response:**
```json
{
  "company": {
    "id": "uuid",
    "ticker": "AAPL",
    "company_name": "Apple Inc.",
    ...
  }
}
```

**Error Responses:**
- `404 Not Found`: Company with specified ticker not found

### GET /api/v1/health
Get overall system health status including scraper performance metrics.

**Response:**
```json
{
  "healthy": true,
  "timestamp": "2025-07-30T10:00:00Z",
  "scraper_health": {
    "is_healthy": true,
    "total_requests": 1500,
    "successful_requests": 1485,
    "failed_requests": 15,
    "success_rate": 0.99,
    "consecutive_failures": 0,
    "last_success_time": "2025-07-30T09:55:00Z",
    "recent_failures": [],
    "health_issues": [],
    "recommended_actions": []
  }
}
```

**Error Response (503 Service Unavailable):**
```json
{
  "healthy": false,
  "timestamp": "2025-07-30T10:00:00Z",
  "error": "scraper health check failed: High failure rate detected",
  "scraper_health": {
    "is_healthy": false,
    "health_issues": ["High failure rate detected (>20%)"],
    "recommended_actions": ["Check OxyLabs connectivity and credentials"]
  }
}
```

### GET /api/v1/health/scraper
Get detailed scraper health metrics and failure analysis.

**Response:**
```json
{
  "health_status": {
    "is_healthy": false,
    "total_requests": 100,
    "successful_requests": 75,
    "failed_requests": 25,
    "success_rate": 0.75,
    "consecutive_failures": 3,
    "last_failure_time": "2025-07-30T09:58:00Z",
    "last_success_time": "2025-07-30T09:55:00Z",
    "recent_failures": [
      {
        "timestamp": "2025-07-30T09:58:00Z",
        "ticker": "ABCD",
        "error": "timeout error",
        "url": "https://www.otcmarkets.com/stock/ABCD/overview"
      }
    ],
    "health_issues": [
      "Frequent timeout errors detected"
    ],
    "recommended_actions": [
      "Consider increasing request timeouts or reducing concurrency"
    ]
  },
  "timestamp": "2025-07-30T10:00:00Z"
}
```

### POST /api/v1/health/scraper/reset
Reset the scraper health monitor (Admin only).

**Response:**
```json
{
  "message": "Scraper health monitor reset successfully",
  "timestamp": "2025-07-30T10:00:00Z"
}
```

**Error Responses:**
- `403 Forbidden`: Non-admin users cannot reset health monitor

## Health Monitoring

The scraper health monitoring system tracks:

### Health Metrics
- **Success Rate**: Percentage of successful scraping operations
- **Consecutive Failures**: Number of consecutive failed operations
- **Recent Failures**: Last 50 failure events with details
- **Failure Patterns**: Automatic categorization of error types

### Health Issues Detection
- **High Failure Rate**: >20% failure rate triggers alert
- **Consecutive Failures**: >5 consecutive failures triggers alert
- **No Recent Success**: No successful requests in last hour
- **Pattern Analysis**: Detects timeout, rate limit, auth, and network issues

### Recommended Actions
The system provides specific recommendations based on detected issues:
- **Timeouts**: Increase timeouts or reduce concurrency
- **Rate Limits**: Implement exponential backoff
- **Authentication**: Check OxyLabs credentials
- **Network**: Verify connectivity and DNS

## Job Status Values

- `pending`: Job has been created but not started
- `running`: Job is currently processing tickers
- `completed`: Job has finished successfully
- `failed`: Job encountered an error and stopped

## Rate Limits

The scraping service implements rate limiting to comply with OTC Markets terms of service:
- Standard processing: ~5 concurrent requests
- Optimized processing: Used automatically for uploads >10 tickers

## Error Handling

All API endpoints return JSON error responses with appropriate HTTP status codes:

```json
{
  "error": "Description of the error"
}
```

Common error scenarios:
- Invalid or missing authentication tokens
- Malformed CSV files
- Server errors during scraping
- Database connectivity issues