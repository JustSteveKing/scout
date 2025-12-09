package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/juststeveking/scout/internal/config"
	"github.com/juststeveking/scout/internal/notify"
)

// Monitor orchestrates health checks for all services
type Monitor struct {
	Config          *config.Config
	checkers        map[string]Checker
	results         chan Result
	done            chan struct{}
	notifier        *notify.Notifier
	serviceStatuses map[string]Status
	muStatusLock    sync.RWMutex
}

// NewMonitor creates a new monitor instance
func NewMonitor(cfg *config.Config) (*Monitor, error) {
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration: %w", err)
	}

	checkers := map[string]Checker{
		"http":    NewHTTPChecker(timeout),
		"tcp":     NewTCPChecker(timeout),
		"tls":     NewTLSChecker(timeout),
		"dns":     NewDNSChecker(timeout),
		"latency": NewLatencyChecker(timeout),
	}

	return &Monitor{
		Config:          cfg,
		checkers:        checkers,
		results:         make(chan Result, len(cfg.Services)*2),
		done:            make(chan struct{}),
		notifier:        notify.NewNotifier(true),
		serviceStatuses: make(map[string]Status),
	}, nil
}

// Start begins monitoring all services
func (m *Monitor) Start(ctx context.Context) {
	defer func() {
		close(m.results)
		close(m.done)
		m.closeCheckers()
	}()

	checkInterval, err := time.ParseDuration(m.Config.CheckInterval)
	if err != nil {
		checkInterval = 30 * time.Second
	}

	// Initialize service statuses so first failure triggers a notification
	m.muStatusLock.Lock()
	for _, service := range m.Config.Services {
		m.serviceStatuses[service.Name] = StatusUnknown
	}
	m.muStatusLock.Unlock()

	// Initial check
	m.checkAll(ctx)

	// Start periodic checks
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

// checkAll performs health checks on all services concurrently
func (m *Monitor) checkAll(ctx context.Context) {
	var wg sync.WaitGroup

	for _, service := range m.Config.Services {
		wg.Add(1)
		go func(svc config.Service) {
			defer wg.Done()
			m.checkService(ctx, svc)
		}(service)
	}

	wg.Wait()
}

// AddService adds a new service to the monitor and triggers an immediate check
func (m *Monitor) AddService(ctx context.Context, service config.Service) {
	// The service should already be added to the config object referenced by m.Config
	// We just need to trigger an immediate check
	go m.checkService(ctx, service)
}

// checkService performs a health check on a single service
func (m *Monitor) checkService(ctx context.Context, service config.Service) {
	// Send checking status
	select {
	case m.results <- Result{
		ServiceName: service.Name,
		Status:      StatusChecking,
		CheckedAt:   time.Now(),
	}:
	case <-ctx.Done():
		return
	}

	// Determine which checker to use
	checkerType := service.Type
	if checkerType == "" {
		checkerType = "http" // Default to HTTP
	}

	checker, exists := m.checkers[checkerType]
	if !exists {
		m.results <- Result{
			ServiceName: service.Name,
			Status:      StatusUnknown,
			Error:       fmt.Errorf("unknown checker type: %s", checkerType),
			CheckedAt:   time.Now(),
		}
		return
	}

	// Perform the check with retry logic
	var result Result
	retries := m.Config.RetryAttempts
	if retries < 1 {
		retries = 1
	}

	for attempt := 0; attempt < retries; attempt++ {
		result = checker.Check(ctx, service)

		if result.Status == StatusHealthy {
			break
		}

		// Wait before retry (except on last attempt)
		if attempt < retries-1 {
			time.Sleep(time.Second)
		}
	}

	// Track status change and send notification if needed
	m.muStatusLock.Lock()
	previousStatus := m.serviceStatuses[result.ServiceName]
	m.serviceStatuses[result.ServiceName] = result.Status
	m.muStatusLock.Unlock()

	// Send notification on status change (but not on initial Checking status)
	if previousStatus != result.Status && result.Status != StatusChecking {
		// Only notify on actual health status changes, not Unknown->Checking
		if (previousStatus != StatusUnknown && previousStatus != StatusChecking) ||
			(result.Status == StatusHealthy || result.Status == StatusUnhealthy) {
			notifyResult := notify.CheckResult{
				ServiceName:  result.ServiceName,
				Status:       notify.Status(result.Status),
				ResponseTime: result.ResponseTime,
				StatusCode:   result.StatusCode,
				Error:        result.Error,
				CheckedAt:    result.CheckedAt,
				Message:      result.Message,
			}
			_ = m.notifier.NotifyStatusChange(notifyResult, notify.Status(previousStatus))
		}
	}

	// Send result
	select {
	case m.results <- result:
	case <-ctx.Done():
		return
	}
}

// Results returns the channel for receiving check results
func (m *Monitor) Results() <-chan Result {
	return m.results
}

// Done returns a channel that's closed when monitoring stops
func (m *Monitor) Done() <-chan struct{} {
	return m.done
}

// closeCheckers closes all checker resources
func (m *Monitor) closeCheckers() {
	for _, checker := range m.checkers {
		if httpChecker, ok := checker.(*HTTPChecker); ok {
			httpChecker.Close()
		}
		if latencyChecker, ok := checker.(*LatencyChecker); ok {
			latencyChecker.Close()
		}
	}
}
