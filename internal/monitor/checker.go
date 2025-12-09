package monitor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/juststeveking/scout/internal/config"
)

// Checker defines the interface for health checking
type Checker interface {
	Check(ctx context.Context, service config.Service) Result
}

// HTTPChecker performs HTTP-based health checks
type HTTPChecker struct {
	client *http.Client
}

// NewHTTPChecker creates a new HTTP checker
func NewHTTPChecker(timeout time.Duration) *HTTPChecker {
	return &HTTPChecker{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		},
	}
}

// Close closes the HTTP client's connection pool
func (h *HTTPChecker) Close() {
	if h.client != nil && h.client.Transport != nil {
		h.client.CloseIdleConnections()
	}
}

// Check performs an HTTP health check
func (h *HTTPChecker) Check(ctx context.Context, service config.Service) Result {
	result := Result{
		ServiceName: service.Name,
		Status:      StatusChecking,
		CheckedAt:   time.Now(),
	}

	// Build the full URL
	url := service.URL
	if service.HealthEndpoint != "" {
		url = strings.TrimRight(url, "/") + service.HealthEndpoint
	}

	// Default to GET if no method specified
	method := service.Method
	if method == "" {
		method = "GET"
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("failed to create request: %w", err)
		return result
	}

	// Add headers
	for key, value := range service.Headers {
		req.Header.Set(key, value)
	}

	// Perform the request
	start := time.Now()
	resp, err := h.client.Do(req)
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = err
		result.Message = "Connection failed"
		return result
	}
	defer resp.Body.Close()
	defer io.ReadAll(resp.Body)

	result.StatusCode = resp.StatusCode

	// Check if status code matches expected
	expectedStatus := service.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	if resp.StatusCode == expectedStatus {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Expected %d, got %d", expectedStatus, resp.StatusCode)
	}

	return result
}

// TCPChecker performs TCP connection checks
type TCPChecker struct {
	timeout time.Duration
}

// NewTCPChecker creates a new TCP checker
func NewTCPChecker(timeout time.Duration) *TCPChecker {
	return &TCPChecker{
		timeout: timeout,
	}
}

// Check performs a TCP connection check
func (t *TCPChecker) Check(ctx context.Context, service config.Service) Result {
	result := Result{
		ServiceName: service.Name,
		Status:      StatusChecking,
		CheckedAt:   time.Now(),
	}

	start := time.Now()

	conn, err := net.DialTimeout("tcp", service.URL, t.timeout)
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = err
		result.Message = "Connection refused"
		return result
	}

	conn.Close()
	result.Status = StatusHealthy
	result.Message = "Port open"

	return result
}
