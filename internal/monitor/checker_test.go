package monitor

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
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
