package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/juststeveking/scout/internal/config"
	"github.com/juststeveking/scout/internal/monitor"
)

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Always update window size
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
	}

	// Handle form updates if form is active
	if m.showForm {
		// Check for cancellation
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.String() == "esc" {
				m.showForm = false
				m.form = nil
				return m, nil
			}
		}

		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			// Process form data
			status, _ := strconv.Atoi(m.formData.ExpectedStatus)
			if status == 0 {
				status = 200
			}

			newService := config.Service{
				Name:           m.formData.Name,
				URL:            m.formData.URL,
				HealthEndpoint: m.formData.HealthEndpoint,
				Method:         m.formData.Method,
				ExpectedStatus: status,
			}

			// Handle authentication
			if m.formData.AuthType != "" {
				newService.Auth = &config.Auth{
					Type:     m.formData.AuthType,
					Token:    m.formData.AuthToken,
					Username: m.formData.AuthUsername,
					Password: m.formData.AuthPassword,
				}
			}

			// Parse custom headers
			if m.formData.Headers != "" {
				headers := parseHeadersFromTUI(m.formData.Headers)
				newService.Headers = headers
			}

			// Parse JSON assertions
			if m.formData.JSONAssertions != "" {
				assertions := parseJSONAssertionsFromTUI(m.formData.JSONAssertions)
				newService.JSONAssertions = assertions
			}

			// Add to config
			if err := m.monitor.Config.AddService(newService); err == nil {
				// Save config
				config.SaveConfig(m.monitor.Config)

				// Immediately surface the new service in the dashboard as "checking"
				checks := m.buildCheckLabels(newService)
				placeholder := ServiceState{
					Name:        newService.Name,
					Status:      monitor.StatusChecking,
					IsChecking:  true,
					LastChecked: time.Now(),
					Checks:      checks,
				}
				replaced := false
				for i := range m.services {
					if m.services[i].Name == newService.Name {
						m.services[i] = placeholder
						replaced = true
						break
					}
				}
				if !replaced {
					m.services = append(m.services, placeholder)
				}
				m.clampSelection()

				// Add to monitor (will send real results)
				m.monitor.AddService(context.Background(), newService)
			}

			m.showForm = false
			m.form = nil
			return m, cmd
		}

		if m.form.State == huh.StateAborted {
			m.showForm = false
			m.form = nil
			return m, cmd
		}

		return m, cmd
	}

	// Handle detail modal interactions
	if m.showDetail {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "esc", "enter":
				m.showDetail = false
				return m, nil
			}
		}
		// When detail is open, ignore other input
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			if m.monitorCancel != nil {
				m.monitorCancel()
			}
			return m, tea.Quit
		case "n":
			m.showForm = true
			m.initAddServiceForm()
			return m, m.form.Init()
		case "enter":
			if len(m.services) > 0 {
				m.detailName = m.getSelectedName()
				m.showDetail = true
			}
		case "left", "h":
			m.moveSelection(-1)
		case "right", "l":
			m.moveSelection(1)
		case "up", "k":
			m.moveSelection(-1)
		case "down", "j":
			m.moveSelection(1)
		case "tab":
			m.moveSelection(1)
		case "shift+tab":
			m.moveSelection(-1)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case resultMsg:
		m.updateServiceState(monitor.Result(msg))
		return m, waitForResults(m.monitor)

	case spinner.TickMsg:
		// Update all active spinners
		var cmds []tea.Cmd
		for name, s := range m.spinners {
			updated, cmd := s.Update(msg)
			m.spinners[name] = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case tickMsg:
		return m, doTick()
	}

	return m, nil
}

// initAddServiceForm initializes the form for adding a new service
func (m *Model) initAddServiceForm() {
	m.formData = &FormData{
		Method:         "GET",
		ExpectedStatus: "200",
		AuthType:       "bearer",
	}
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Service Name").
				Value(&m.formData.Name),
			huh.NewInput().
				Title("Service URL").
				Value(&m.formData.URL),
			huh.NewInput().
				Title("Health Endpoint (optional)").
				Value(&m.formData.HealthEndpoint),
			huh.NewSelect[string]().
				Title("HTTP Method").
				Options(
					huh.NewOption("GET", "GET"),
					huh.NewOption("POST", "POST"),
					huh.NewOption("PUT", "PUT"),
					huh.NewOption("DELETE", "DELETE"),
				).
				Value(&m.formData.Method),
			huh.NewInput().
				Title("Expected Status Code").
				Value(&m.formData.ExpectedStatus),
		).Title("Service Details (Esc to cancel)"),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Auth Type").
				Options(
					huh.NewOption("None", ""),
					huh.NewOption("Bearer Token", "bearer"),
					huh.NewOption("Basic Auth", "basic"),
				).
				Value(&m.formData.AuthType),
			huh.NewInput().
				Title("Bearer Token (if using bearer auth)").
				Value(&m.formData.AuthToken),
			huh.NewInput().
				Title("Username (if using basic auth)").
				Value(&m.formData.AuthUsername),
			huh.NewInput().
				Title("Password (if using basic auth)").
				Value(&m.formData.AuthPassword),
		).Title("Authentication (Optional)"),
		huh.NewGroup(
			huh.NewInput().
				Title("Custom Headers (key:value,key:value)").
				Value(&m.formData.Headers),
			huh.NewInput().
				Title("JSON Assertions (path:value:operator,...)").
				Description("Example: status:ok:==,uptime:0:>").
				Value(&m.formData.JSONAssertions),
		).Title("Advanced (Optional)"),
	).WithTheme(huh.ThemeCatppuccin()).WithWidth(80).WithShowHelp(true)
}

// updateServiceState updates or adds a service state based on a result
func (m *Model) updateServiceState(result monitor.Result) {
	// Find existing service or create new one
	found := false
	isChecking := result.Status == monitor.StatusChecking

	checks := []string{}
	if cfg := m.getServiceConfig(result.ServiceName); cfg != nil {
		checks = m.buildCheckLabels(*cfg)
	}

	for i, svc := range m.services {
		if svc.Name == result.ServiceName {
			m.services[i] = ServiceState{
				Name:         result.ServiceName,
				Status:       result.Status,
				ResponseTime: result.ResponseTime,
				Message:      result.Message,
				LastChecked:  result.CheckedAt,
				StatusCode:   result.StatusCode,
				Error:        result.Error,
				IsChecking:   isChecking,
				Checks:       checks,
			}
			found = true
			break
		}
	}

	if !found {
		m.services = append(m.services, ServiceState{
			Name:         result.ServiceName,
			Status:       result.Status,
			ResponseTime: result.ResponseTime,
			Message:      result.Message,
			LastChecked:  result.CheckedAt,
			StatusCode:   result.StatusCode,
			Error:        result.Error,
			IsChecking:   isChecking,
			Checks:       checks,
		})
	}

	m.clampSelection()

	// Manage spinners
	if isChecking {
		// Create spinner if it doesn't exist
		if _, exists := m.spinners[result.ServiceName]; !exists {
			s := spinner.New()
			s.Spinner = spinner.MiniDot
			s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")) // Gold
			m.spinners[result.ServiceName] = s
		}
	} else {
		// Remove spinner when done checking
		delete(m.spinners, result.ServiceName)
	}
}

// parseHeadersFromTUI parses headers from TUI format (key:value,key:value)
func parseHeadersFromTUI(headerStr string) map[string]string {
	headers := make(map[string]string)
	if headerStr == "" {
		return headers
	}

	pairs := strings.Split(headerStr, ",")
	for _, pair := range pairs {
		kv := strings.Split(strings.TrimSpace(pair), ":")
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headers
}

// parseJSONAssertionsFromTUI parses JSON assertions from TUI format (path:value:operator,...)
func parseJSONAssertionsFromTUI(assertionStr string) []config.JSONAssertion {
	var assertions []config.JSONAssertion
	if assertionStr == "" {
		return assertions
	}

	pairs := strings.Split(assertionStr, ",")
	for _, pair := range pairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) >= 3 {
			assertion := config.JSONAssertion{
				Path:     parts[0],
				Value:    parseJSONValueFromTUI(parts[1]),
				Operator: parts[2],
			}
			assertions = append(assertions, assertion)
		}
	}
	return assertions
}

// parseJSONValueFromTUI attempts to parse a string into a JSON-compatible value
func parseJSONValueFromTUI(s string) interface{} {
	s = strings.TrimSpace(s)
	// Try to parse as boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Try to parse as number
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Return as string
	return s
}

// getServiceConfig returns the config for a service name
func (m *Model) getServiceConfig(name string) *config.Service {
	if m.monitor == nil || m.monitor.Config == nil {
		return nil
	}
	for i := range m.monitor.Config.Services {
		if m.monitor.Config.Services[i].Name == name {
			return &m.monitor.Config.Services[i]
		}
	}
	return nil
}

// buildCheckLabels returns human-readable labels for enabled checks
func (m *Model) buildCheckLabels(svc config.Service) []string {
	labels := []string{}

	// Base type
	switch strings.ToLower(svc.Type) {
	case "tcp":
		labels = append(labels, "TCP")
	case "tls":
		labels = append(labels, "TLS")
	case "dns":
		labels = append(labels, "DNS")
	case "latency":
		labels = append(labels, "Latency")
	default:
		labels = append(labels, "HTTP")
	}

	if svc.TLSCheck {
		labels = append(labels, "TLS")
	}
	if svc.DNSCheck {
		labels = append(labels, "DNS")
	}
	if svc.TCPPingCheck {
		labels = append(labels, "TCP")
	}
	if svc.LatencyCheck {
		threshold := svc.LatencyThreshold
		label := "Latency"
		if threshold > 0 {
			label = fmt.Sprintf("Latency≤%dms", threshold)
		}
		labels = append(labels, label)
	}
	if len(svc.JSONAssertions) > 0 {
		labels = append(labels, "JSON")
	}

	return dedupe(labels)
}

// dedupe removes duplicates while preserving order
func dedupe(items []string) []string {
	seen := make(map[string]bool)
	out := []string{}
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// moveSelection moves the selected index with wrap-around
func (m *Model) moveSelection(delta int) {
	if len(m.services) == 0 {
		return
	}
	m.selectedIndex = (m.selectedIndex + delta) % len(m.services)
	if m.selectedIndex < 0 {
		m.selectedIndex += len(m.services)
	}
}

// getSelectedName returns the currently selected service name
func (m *Model) getSelectedName() string {
	if len(m.services) == 0 {
		return ""
	}
	if m.selectedIndex >= len(m.services) {
		m.selectedIndex = len(m.services) - 1
	}
	return m.services[m.selectedIndex].Name
}

// clampSelection ensures selection stays within range
func (m *Model) clampSelection() {
	if len(m.services) == 0 {
		m.selectedIndex = 0
		return
	}
	if m.selectedIndex >= len(m.services) {
		m.selectedIndex = len(m.services) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
}
