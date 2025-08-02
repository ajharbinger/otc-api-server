package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Company represents a company record
type Company struct {
	ID               uuid.UUID `json:"id" db:"id"`
	Ticker           string    `json:"ticker" db:"ticker"`
	CompanyName      string    `json:"company_name" db:"company_name"`
	MarketTier       string    `json:"market_tier" db:"market_tier"`
	QuoteStatus      string    `json:"quote_status" db:"quote_status"`
	TradingVolume    int64     `json:"trading_volume" db:"trading_volume"`
	Website          string    `json:"website" db:"website"`
	Description      string    `json:"description" db:"description"`
	Officers         Officers  `json:"officers" db:"officers"`
	Address          Address   `json:"address" db:"address"`
	TransferAgent    string    `json:"transfer_agent" db:"transfer_agent"`
	Auditor          string    `json:"auditor" db:"auditor"`
	Last10KDate      *time.Time `json:"last_10k_date" db:"last_10k_date"`
	Last10QDate      *time.Time `json:"last_10q_date" db:"last_10q_date"`
	LastFilingDate   *time.Time `json:"last_filing_date" db:"last_filing_date"`
	ProfileVerified  bool      `json:"profile_verified" db:"profile_verified"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// Officers represents company officers as JSON
type Officers []Officer

// Officer represents a company officer
type Officer struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Location string `json:"location"`
}

// Address represents company address as JSON
type Address struct {
	Street   string `json:"street"`
	City     string `json:"city"`
	State    string `json:"state"`
	Country  string `json:"country"`
	PostCode string `json:"post_code"`
}

// Value implements driver.Valuer for Officers
func (o Officers) Value() (driver.Value, error) {
	return json.Marshal(o)
}

// Scan implements sql.Scanner for Officers
func (o *Officers) Scan(value interface{}) error {
	if value == nil {
		*o = Officers{}
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into Officers", value)
	}
	
	return json.Unmarshal(bytes, o)
}

// Value implements driver.Valuer for Address
func (a Address) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan implements sql.Scanner for Address
func (a *Address) Scan(value interface{}) error {
	if value == nil {
		*a = Address{}
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into Address", value)
	}
	
	return json.Unmarshal(bytes, a)
}

// ScrapedData represents raw scraped data from OTC Markets
type ScrapedData struct {
	Ticker      string                 `json:"ticker"`
	Overview    map[string]interface{} `json:"overview"`
	Financials  map[string]interface{} `json:"financials"`
	Disclosure  map[string]interface{} `json:"disclosure"`
	ScrapedAt   time.Time             `json:"scraped_at"`
	Errors      []string              `json:"errors,omitempty"`
}

// ScrapeJob represents a scraping job
type ScrapeJob struct {
	ID                uuid.UUID `json:"id" db:"id"`
	Status            string    `json:"status" db:"status"`
	TotalTickers      int       `json:"total_tickers" db:"total_tickers"`
	ProcessedTickers  int       `json:"processed_tickers" db:"processed_tickers"`
	FailedTickers     int       `json:"failed_tickers" db:"failed_tickers"`
	StartedBy         uuid.UUID `json:"started_by" db:"started_by"`
	StartedAt         time.Time `json:"started_at" db:"started_at"`
	CompletedAt       *time.Time `json:"completed_at" db:"completed_at"`
	ErrorMessage      string    `json:"error_message" db:"error_message"`
}

// ScrapeJobStatus represents scrape job status values
type ScrapeJobStatus string

const (
	ScrapeJobPending   ScrapeJobStatus = "pending"
	ScrapeJobRunning   ScrapeJobStatus = "running"
	ScrapeJobCompleted ScrapeJobStatus = "completed"
	ScrapeJobFailed    ScrapeJobStatus = "failed"
)