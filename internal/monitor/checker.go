package monitor

import (
	"context"
	"crypto/tls"
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
func (h *HTTPChecker) validateJSONAssertions(body string, assertions []config.JSONAssertion, _ Result) error {
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

// TLSChecker checks TLS certificate expiry
type TLSChecker struct {
	timeout time.Duration
}

// NewTLSChecker creates a new TLS checker
func NewTLSChecker(timeout time.Duration) *TLSChecker {
	return &TLSChecker{
		timeout: timeout,
	}
}

// Check performs a TLS certificate expiry check
func (t *TLSChecker) Check(ctx context.Context, service config.Service) Result {
	result := Result{
		ServiceName: service.Name,
		Status:      StatusChecking,
		CheckedAt:   time.Now(),
	}

	// Extract host from URL
	host := service.URL
	if strings.Contains(host, "://") {
		host = strings.Split(host, "://")[1]
	}
	if strings.Contains(host, "/") {
		host = strings.Split(host, "/")[0]
	}

	// Add default HTTPS port if not specified
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}

	// Use TLS dial with context
	start := time.Now()
	tlsConn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: t.timeout},
		"tcp",
		host,
		&tls.Config{InsecureSkipVerify: false},
	)
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("TLS connection failed: %w", err)
		result.Message = "TLS connection failed"
		return result
	}
	defer tlsConn.Close()

	// Get certificate
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("no certificates found")
		result.Message = "No certificates found"
		return result
	}

	cert := certs[0]
	expiryDays := int(time.Until(cert.NotAfter).Hours() / 24)
	warningDays := service.TLSWarningDays
	if warningDays == 0 {
		warningDays = 30 // Default: warn 30 days before expiry
	}

	result.Message = fmt.Sprintf("Certificate expires in %d days", expiryDays)

	if time.Now().After(cert.NotAfter) {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("certificate expired on %s", cert.NotAfter.Format("2006-01-02"))
		return result
	}

	if expiryDays < warningDays {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("certificate expires in %d days (warning threshold: %d days)", expiryDays, warningDays)
		return result
	}

	result.Status = StatusHealthy
	return result
}

// DNSChecker checks DNS resolution
type DNSChecker struct {
	timeout time.Duration
}

// NewDNSChecker creates a new DNS checker
func NewDNSChecker(timeout time.Duration) *DNSChecker {
	return &DNSChecker{
		timeout: timeout,
	}
}

// Check performs a DNS resolution check
func (d *DNSChecker) Check(ctx context.Context, service config.Service) Result {
	result := Result{
		ServiceName: service.Name,
		Status:      StatusChecking,
		CheckedAt:   time.Now(),
	}

	// Extract host from URL
	host := service.URL
	if strings.Contains(host, "://") {
		host = strings.Split(host, "://")[1]
	}
	if strings.Contains(host, "/") {
		host = strings.Split(host, "/")[0]
	}
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	start := time.Now()
	resolver := &net.Resolver{
		PreferGo: true,
	}

	ips, err := resolver.LookupIPAddr(ctx, host)
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("DNS resolution failed: %w", err)
		result.Message = "DNS resolution failed"
		return result
	}

	if len(ips) == 0 {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("no IP addresses found for %s", host)
		result.Message = "No IP addresses found"
		return result
	}

	result.Status = StatusHealthy
	result.Message = fmt.Sprintf("Resolved to %s", ips[0].String())
	return result
}

// LatencyChecker checks response latency
type LatencyChecker struct {
	client *http.Client
}

// NewLatencyChecker creates a new latency checker
func NewLatencyChecker(timeout time.Duration) *LatencyChecker {
	return &LatencyChecker{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Close closes the HTTP client
func (l *LatencyChecker) Close() {
	if l.client != nil && l.client.Transport != nil {
		l.client.CloseIdleConnections()
	}
}

// Check performs an HTTP latency check
func (l *LatencyChecker) Check(ctx context.Context, service config.Service) Result {
	result := Result{
		ServiceName: service.Name,
		Status:      StatusChecking,
		CheckedAt:   time.Now(),
	}

	// Build URL
	url := service.URL
	if service.HealthEndpoint != "" {
		url = strings.TrimRight(url, "/") + service.HealthEndpoint
	}

	method := service.Method
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("failed to create request: %w", err)
		return result
	}

	// Add headers and auth (same as HTTPChecker)
	for key, value := range service.Headers {
		req.Header.Set(key, value)
	}
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

	start := time.Now()
	resp, err := l.client.Do(req)
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Status = StatusUnhealthy
		result.Error = err
		result.Message = "Connection failed"
		return result
	}
	defer resp.Body.Close()
	defer io.ReadAll(resp.Body)

	latencyMs := result.ResponseTime.Milliseconds()
	thresholdMs := int64(service.LatencyThreshold)
	if thresholdMs == 0 {
		thresholdMs = 5000 // Default: 5 seconds
	}

	result.Message = fmt.Sprintf("Latency: %dms", latencyMs)

	if latencyMs > thresholdMs {
		result.Status = StatusUnhealthy
		result.Error = fmt.Errorf("latency %dms exceeds threshold of %dms", latencyMs, thresholdMs)
		return result
	}

	result.Status = StatusHealthy
	return result
}
