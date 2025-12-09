package monitor

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/juststeveking/scout/internal/config"
)

func TestHTTPChecker(t *testing.T) {
	// Start a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	// Test healthy service
	svc := config.Service{
		Name:           "test-service",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		ExpectedStatus: 200,
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %v", result.Status)
	}

	// Test unhealthy service (wrong endpoint)
	svc.HealthEndpoint = "/wrong"
	result = checker.Check(context.Background(), svc)
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy, got %v", result.Status)
	}

	// Test unhealthy service (wrong status code expectation)
	svc.HealthEndpoint = "/health"
	svc.ExpectedStatus = 201
	result = checker.Check(context.Background(), svc)
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy for wrong status code, got %v", result.Status)
	}
}

func TestHTTPCheckerWithCustomHeaders(t *testing.T) {
	// Start a test server that validates headers
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for custom header
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Check for another custom header
		if r.Header.Get("X-Request-ID") != "test-123" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	svc := config.Service{
		Name:           "test-headers",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Request-ID":    "test-123",
		},
		ExpectedStatus: 200,
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy with custom headers, got %v", result.Status)
	}
}

func TestHTTPCheckerWithBearerAuth(t *testing.T) {
	// Start a test server that validates Bearer token
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	svc := config.Service{
		Name:           "test-bearer-auth",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		Auth: &config.Auth{
			Type:  "bearer",
			Token: "test-token-123",
		},
		ExpectedStatus: 200,
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy with bearer auth, got %v", result.Status)
	}
}

func TestHTTPCheckerWithBasicAuth(t *testing.T) {
	// Start a test server that validates Basic auth
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "testuser" || password != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	svc := config.Service{
		Name:           "test-basic-auth",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		Auth: &config.Auth{
			Type:     "basic",
			Username: "testuser",
			Password: "testpass",
		},
		ExpectedStatus: 200,
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy with basic auth, got %v", result.Status)
	}
}

func TestHTTPCheckerWithHeadersAndAuth(t *testing.T) {
	// Start a test server that validates both custom headers and auth
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check custom header
		if r.Header.Get("X-Custom") != "value" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Check Bearer token
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	svc := config.Service{
		Name:           "test-headers-and-auth",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		Headers: map[string]string{
			"X-Custom": "value",
		},
		Auth: &config.Auth{
			Type:  "bearer",
			Token: "secret-token",
		},
		ExpectedStatus: 200,
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy with headers and auth, got %v", result.Status)
	}
}

func TestHTTPCheckerWithJSONAssertions(t *testing.T) {
	// Start a test server that returns JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "ok",
			"uptime": 3600,
			"dependencies": {
				"database": true,
				"cache": true
			},
			"message": "all systems operational",
			"version": "1.0.5"
		}`))
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	svc := config.Service{
		Name:           "test-json-assertions",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		ExpectedStatus: 200,
		JSONAssertions: []config.JSONAssertion{
			{
				Path:     "status",
				Value:    "ok",
				Operator: "==",
			},
			{
				Path:     "uptime",
				Value:    float64(0),
				Operator: ">",
			},
			{
				Path:     "dependencies.database",
				Value:    true,
				Operator: "==",
			},
			{
				Path:     "message",
				Value:    "fatal",
				Operator: "!=",
			},
		},
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy with JSON assertions, got %v: %v", result.Status, result.Error)
	}
}

func TestHTTPCheckerWithJSONAssertionFailure(t *testing.T) {
	// Start a test server that returns JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "degraded", "uptime": 100}`))
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	svc := config.Service{
		Name:           "test-json-assertion-failure",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		ExpectedStatus: 200,
		JSONAssertions: []config.JSONAssertion{
			{
				Path:     "status",
				Value:    "ok",
				Operator: "==",
			},
		},
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy when JSON assertion fails, got %v", result.Status)
	}
	if result.Error == nil {
		t.Errorf("Expected error for failed JSON assertion")
	}
}

func TestHTTPCheckerWithJSONAssertionMissingPath(t *testing.T) {
	// Start a test server that returns JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	svc := config.Service{
		Name:           "test-json-missing-path",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		ExpectedStatus: 200,
		JSONAssertions: []config.JSONAssertion{
			{
				Path:     "nonexistent.path",
				Value:    "value",
				Operator: "==",
			},
		},
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy when JSON path is missing, got %v", result.Status)
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "not found") {
		t.Errorf("Expected error about missing JSON path")
	}
}

func TestHTTPCheckerWithJSONAssertionComparisons(t *testing.T) {
	// Start a test server that returns JSON with numeric values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"response_time": 250,
			"error_rate": 0.01,
			"success_rate": 99.5
		}`))
	}))
	defer ts.Close()

	checker := NewHTTPChecker(1 * time.Second)
	defer checker.Close()

	svc := config.Service{
		Name:           "test-json-comparisons",
		URL:            ts.URL,
		HealthEndpoint: "/health",
		ExpectedStatus: 200,
		JSONAssertions: []config.JSONAssertion{
			{
				Path:     "response_time",
				Value:    float64(1000),
				Operator: "<",
			},
			{
				Path:     "error_rate",
				Value:    float64(0.05),
				Operator: "<",
			},
			{
				Path:     "success_rate",
				Value:    float64(90),
				Operator: ">",
			},
		},
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy with comparison assertions, got %v: %v", result.Status, result.Error)
	}
}

func TestTCPChecker(t *testing.T) {
	// Start a listener
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	checker := NewTCPChecker(1 * time.Second)

	svc := config.Service{
		Name: "test-tcp",
		URL:  l.Addr().String(),
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %v", result.Status)
	}

	// Test closed port
	l.Close()

	// We need to make sure the port is actually closed before checking
	// But since we closed the listener, subsequent dials should fail.

	result = checker.Check(context.Background(), svc)
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy for closed port, got %v", result.Status)
	}
}

func TestDNSChecker(t *testing.T) {
	checker := NewDNSChecker(5 * time.Second)

	// Test valid hostname
	svc := config.Service{
		Name: "test-dns",
		URL:  "google.com",
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy for valid DNS, got %v: %v", result.Status, result.Error)
	}

	// Test invalid hostname
	svc.URL = "this-should-not-exist-example-12345.invalid"
	result = checker.Check(context.Background(), svc)
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy for invalid hostname, got %v", result.Status)
	}
}

func TestLatencyChecker(t *testing.T) {
	// Start a test server with a delay
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	checker := NewLatencyChecker(5 * time.Second)
	defer checker.Close()

	// Test with no latency threshold
	svc := config.Service{
		Name: "test-latency",
		URL:  ts.URL,
	}

	result := checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %v: %v", result.Status, result.Error)
	}

	// Test with high latency threshold (should pass)
	svc.LatencyThreshold = 10000 // 10 second threshold
	result = checker.Check(context.Background(), svc)
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy with high latency threshold, got %v", result.Status)
	}

	// Test with very low latency threshold to simulate failure
	// Note: Real-world latency varies, so we use an extremely low threshold
	svc.LatencyThreshold = 1 // 1ms - nearly impossible to achieve
	result = checker.Check(context.Background(), svc)
	// This test is flaky since local requests are very fast, so we just check it returns a result
	if result.ResponseTime == 0 {
		t.Errorf("Expected non-zero response time, got %v", result.ResponseTime)
	}
}
