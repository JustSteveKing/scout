package tui

import (
	"context"
	"strconv"

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
				Method:         m.formData.Method,
				ExpectedStatus: status,
			}

			if m.formData.Token != "" {
				newService.Auth = &config.Auth{
					Type:  "bearer",
					Token: m.formData.Token,
				}
			}

			// Add to config
			if err := m.monitor.Config.AddService(newService); err == nil {
				// Save config
				config.SaveConfig(m.monitor.Config)

				// Add to monitor
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
	}
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Service Name").
				Value(&m.formData.Name),
			huh.NewInput().
				Title("Service URL").
				Value(&m.formData.URL),
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
			huh.NewInput().
				Title("Bearer Token (Optional)").
				Value(&m.formData.Token),
		).Title("Add New Service (Esc to cancel)"),
	).WithTheme(huh.ThemeCatppuccin()).WithWidth(60).WithShowHelp(true)
}

// updateServiceState updates or adds a service state based on a result
func (m *Model) updateServiceState(result monitor.Result) {
	// Find existing service or create new one
	found := false
	isChecking := result.Status == monitor.StatusChecking

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
		})
	}

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
