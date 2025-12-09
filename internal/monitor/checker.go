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
	"github.com/tidwall/gjson"
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

	// Add custom headers
	for key, value := range service.Headers {
		req.Header.Set(key, value)
	}

	// Add authentication headers
	if service.Auth != nil {
		switch strings.ToLower(service.Auth.Type) {
		case "bearer":
			if service.Auth.Token != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", service.Auth.Token))
			}
		case "basic":
			if service.Auth.Username != "" && service.Auth.Password != "" {
				req.SetBasicAuth(service.Auth.Username, service.Auth.Password)
			}
		}
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

	result.StatusCode = resp.StatusCode

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("failed to read response body: %w", err)
		return result
	}

	// Check if status code matches expected
	expectedStatus := service.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	if resp.StatusCode != expectedStatus {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Expected %d, got %d", expectedStatus, resp.StatusCode)
		return result
	}

	// If there are JSON assertions, validate them
	if len(service.JSONAssertions) > 0 {
		if err := h.validateJSONAssertions(string(body), service.JSONAssertions, result); err != nil {
			result.Status = StatusUnhealthy
			result.Error = err
			return result
		}
	}

	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)

	return result
}

// validateJSONAssertions checks JSON assertions against the response body
func (h *HTTPChecker) validateJSONAssertions(body string, assertions []config.JSONAssertion, result Result) error {
	for _, assertion := range assertions {
		value := gjson.Get(body, assertion.Path)

		if !value.Exists() {
			return fmt.Errorf("JSON path '%s' not found in response", assertion.Path)
		}

		if !h.compareValue(value, assertion.Value, assertion.Operator) {
			return fmt.Errorf("JSON assertion failed: %s %s %v, got %v", assertion.Path, assertion.Operator, assertion.Value, value.Value())
		}
	}
	return nil
}

// compareValue compares a gjson.Result with an expected value using the specified operator
func (h *HTTPChecker) compareValue(actual gjson.Result, expected interface{}, operator string) bool {
	switch strings.ToLower(operator) {
	case "==", "equals":
		return h.jsonValueEquals(actual, expected)
	case "!=", "not_equals":
		return !h.jsonValueEquals(actual, expected)
	case ">":
		return h.jsonGreaterThan(actual, expected)
	case "<":
		return h.jsonLessThan(actual, expected)
	case ">=":
		return h.jsonGreaterOrEqual(actual, expected)
	case "<=":
		return h.jsonLessOrEqual(actual, expected)
	case "contains":
		return h.jsonContains(actual, expected)
	default:
		return false
	}
}

// jsonValueEquals checks if JSON values are equal
func (h *HTTPChecker) jsonValueEquals(actual gjson.Result, expected interface{}) bool {
	switch v := expected.(type) {
	case string:
		return actual.String() == v
	case float64:
		return actual.Float() == v
	case bool:
		return actual.Bool() == v
	case nil:
		return !actual.Exists()
	default:
		return false
	}
}

// jsonGreaterThan checks if actual > expected
func (h *HTTPChecker) jsonGreaterThan(actual gjson.Result, expected interface{}) bool {
	if v, ok := expected.(float64); ok {
		return actual.Float() > v
	}
	return false
}

// jsonLessThan checks if actual < expected
func (h *HTTPChecker) jsonLessThan(actual gjson.Result, expected interface{}) bool {
	if v, ok := expected.(float64); ok {
		return actual.Float() < v
	}
	return false
}

// jsonGreaterOrEqual checks if actual >= expected
func (h *HTTPChecker) jsonGreaterOrEqual(actual gjson.Result, expected interface{}) bool {
	if v, ok := expected.(float64); ok {
		return actual.Float() >= v
	}
	return false
}

// jsonLessOrEqual checks if actual <= expected
func (h *HTTPChecker) jsonLessOrEqual(actual gjson.Result, expected interface{}) bool {
	if v, ok := expected.(float64); ok {
		return actual.Float() <= v
	}
	return false
}

// jsonContains checks if actual string contains expected substring
func (h *HTTPChecker) jsonContains(actual gjson.Result, expected interface{}) bool {
	if v, ok := expected.(string); ok {
		return strings.Contains(actual.String(), v)
	}
	return false
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
