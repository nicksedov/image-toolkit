package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Status represents the OCR service status
type Status struct {
	HealthStatus HealthStatus `json:"healthStatus"`
	Error        string       `json:"error,omitempty"`
	LastCheck    time.Time    `json:"lastCheck"`
}

// HealthStatus represents the health status of OCR service
type HealthStatus string

const (
	HealthStatusUnknown  HealthStatus = "unknown"
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// Client is an interface for OCR classifier service
type Client interface {
	// CheckHealth checks if OCR service is available
	CheckHealth(ctx context.Context) (HealthStatus, error)
	// GetStatus returns the current OCR status
	GetStatus() Status
	// StartHealthCheck starts the periodic health check in background
	StartHealthCheck(intervalSeconds int)
	// StopHealthCheck stops the periodic health check
	StopHealthCheck()
}

type clientImpl struct {
	baseURL      string
	httpClient   *http.Client
	status       Status
	stopCheck    chan struct{}
	isRunning    bool
}

// NewClient creates a new OCR client
func NewClient(host string, port string) Client {
	baseURL := fmt.Sprintf("http://%s:%s/ocr-classifier/api", host, port)
	return &clientImpl{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		status: Status{
			HealthStatus: HealthStatusUnknown,
			LastCheck:    time.Now(),
		},
		stopCheck: make(chan struct{}),
	}
}

// CheckHealth checks if OCR service is available
func (c *clientImpl) CheckHealth(ctx context.Context) (HealthStatus, error) {
	url := fmt.Sprintf("%s/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HealthStatusUnhealthy, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return HealthStatusUnhealthy, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return HealthStatusUnhealthy, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return HealthStatusUnhealthy, fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		return HealthStatusUnhealthy, fmt.Errorf("failed to parse response: %w", err)
	}

	if result["status"] == "ok" {
		return HealthStatusHealthy, nil
	}

	return HealthStatusUnhealthy, fmt.Errorf("health check returned non-OK status")
}

// GetStatus returns the current OCR status
func (c *clientImpl) GetStatus() Status {
	c.status.LastCheck = time.Now()
	return c.status
}

// StartHealthCheck starts the periodic health check in background
func (c *clientImpl) StartHealthCheck(intervalSeconds int) {
	if c.isRunning {
		return
	}

	c.isRunning = true
	go c.healthCheckLoop(intervalSeconds)
}

// StopHealthCheck stops the periodic health check
func (c *clientImpl) StopHealthCheck() {
	if !c.isRunning {
		return
	}

	close(c.stopCheck)
	c.isRunning = false
}

func (c *clientImpl) healthCheckLoop(intervalSeconds int) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCheck:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			status, err := c.CheckHealth(ctx)
			cancel()

			c.status.LastCheck = time.Now()
			if err != nil {
				c.status.HealthStatus = HealthStatusUnhealthy
				c.status.Error = err.Error()
			} else {
				c.status.HealthStatus = status
				c.status.Error = ""
			}
		}
	}
}
