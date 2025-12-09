package monitor

import "time"

// Status represents the health status of a service
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusUnknown   Status = "unknown"
	StatusChecking  Status = "checking"
)

// Result represents the result of a health check
type Result struct {
	ServiceName  string
	Status       Status
	ResponseTime time.Duration
	StatusCode   int
	Error        error
	CheckedAt    time.Time
	Message      string
}
