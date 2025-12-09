package notify

import (
	"fmt"
	"time"

	"github.com/martinlindhe/notify"
)

// Status represents a health check status
type Status string

// CheckResult contains the result of a health check
type CheckResult struct {
	ServiceName  string
	Status       Status
	ResponseTime time.Duration
	StatusCode   int
	Error        error
	CheckedAt    time.Time
	Message      string
}

// Notifier sends desktop notifications for health check events
type Notifier struct {
	enabled bool
}

// NewNotifier creates a new notifier instance
func NewNotifier(enabled bool) *Notifier {
	return &Notifier{
		enabled: enabled,
	}
}

// NotifyFailure sends a desktop notification when a service check fails
func (n *Notifier) NotifyFailure(result CheckResult) error {
	if !n.enabled {
		return nil
	}

	title := fmt.Sprintf("⚠️  %s - Health Check Failed", result.ServiceName)
	message := result.Message
	if result.Error != nil {
		message = fmt.Sprintf("%s: %v", result.Message, result.Error)
	}

	notify.Notify("Scout", title, message, "")
	return nil
}

// NotifyRecovery sends a desktop notification when a service recovers
func (n *Notifier) NotifyRecovery(result CheckResult) error {
	if !n.enabled {
		return nil
	}

	title := fmt.Sprintf("✅ %s - Health Check Recovered", result.ServiceName)
	message := fmt.Sprintf("Response time: %s", result.ResponseTime.String())

	notify.Notify("Scout", title, message, "")
	return nil
}

// NotifyStatusChange sends a desktop notification when a service status changes
func (n *Notifier) NotifyStatusChange(result CheckResult, previousStatus Status) error {
	if !n.enabled {
		return nil
	}

	healthyStatus := Status("healthy")
	unhealthyStatus := Status("unhealthy")

	// Service recovered (was unhealthy, now healthy)
	if result.Status == healthyStatus && previousStatus == unhealthyStatus {
		return n.NotifyRecovery(result)
	}

	// Service failed (was healthy or unknown, now unhealthy)
	if result.Status == unhealthyStatus && (previousStatus == healthyStatus || previousStatus == Status("unknown")) {
		return n.NotifyFailure(result)
	}

	return nil
}
