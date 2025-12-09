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
