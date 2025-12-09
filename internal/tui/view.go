package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/juststeveking/scout/internal/monitor"
)

var (
	colorAccent    = lipgloss.Color("#04D9FF") // Neon Cyan
	colorHealthy   = lipgloss.Color("#00FF94") // Neon Green
	colorUnhealthy = lipgloss.Color("#FF0055") // Neon Red
	colorChecking  = lipgloss.Color("#FFD700") // Gold
	colorMuted     = lipgloss.Color("#565f89") // Muted Blue
	colorSubtle    = lipgloss.Color("#24283b") // Dark Blue
	colorCard      = lipgloss.Color("#16161e") // Very Dark Blue
	colorText      = lipgloss.Color("#c0caf5") // Light Blue/White

	// Title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			MarginBottom(1)

	// Subtitle/header style
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			MarginTop(1).
			MarginBottom(1)

	// Status indicators
	healthyStyle = lipgloss.NewStyle().
			Foreground(colorHealthy).
			Bold(true)

	unhealthyStyle = lipgloss.NewStyle().
			Foreground(colorUnhealthy).
			Bold(true)

	checkingStyle = lipgloss.NewStyle().
			Foreground(colorChecking).
			Bold(true)

	// Base card style (border color will be overridden)
	baseCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Background(colorCard).
			Padding(0, 1).
			MarginRight(1).
			MarginBottom(1)

	// Metadata style
	metadataStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(colorUnhealthy)

	// Service name style for grid
	serviceNameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorText)

	// Secondary info style
	secondaryStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

// View renders the TUI with full-screen grid layout
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// Render form if active
	if m.showForm {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAccent).
				Padding(1, 2).
				Render(m.form.View()),
		)
	}

	// Handle initial state when width is not set
	width := m.width
	if width < 40 {
		width = 80
	}

	// Calculate grid dimensions
	cols := 2
	if width > 160 {
		cols = 3
	}
	if width > 200 {
		cols = 4
	}
	cardWidth := (width - 4) / cols
	if cardWidth < 20 {
		cardWidth = 20
		cols = 1
	}

	var b strings.Builder

	// Header - Modern design with stats
	headerContent := m.renderHeader(width, len(m.services))
	b.WriteString(headerContent)
	b.WriteString("\n")

	// Services or loading state
	if len(m.services) == 0 {
		b.WriteString("\n")
		centerText := "⟳ Waiting for health checks..."
		padding := (width - len(centerText)) / 2
		if padding > 0 {
			b.WriteString(strings.Repeat(" ", padding))
		}
		b.WriteString(metadataStyle.Render(centerText))
		b.WriteString("\n")
	} else {
		// Group services by status
		healthy := []ServiceState{}
		unhealthy := []ServiceState{}
		checking := []ServiceState{}

		for _, svc := range m.services {
			if svc.IsChecking {
				checking = append(checking, svc)
			} else if svc.Status == monitor.StatusHealthy {
				healthy = append(healthy, svc)
			} else {
				unhealthy = append(unhealthy, svc)
			}
		}

		// Render checking services in grid
		if len(checking) > 0 {
			b.WriteString("\n" + headerStyle.Render("⟳ Checking ("+fmt.Sprintf("%d", len(checking))+")") + "\n")
			b.WriteString(m.renderServiceGrid(checking, cardWidth, cols))
		}

		// Render healthy services in grid
		if len(healthy) > 0 {
			b.WriteString("\n" + headerStyle.Render("✓ Healthy ("+fmt.Sprintf("%d", len(healthy))+")") + "\n")
			b.WriteString(m.renderServiceGrid(healthy, cardWidth, cols))
		}

		// Render unhealthy services in grid
		if len(unhealthy) > 0 {
			b.WriteString("\n" + headerStyle.Render("✗ Unhealthy ("+fmt.Sprintf("%d", len(unhealthy))+")") + "\n")
			b.WriteString(m.renderServiceGrid(unhealthy, cardWidth, cols))
		}
	}

	// Footer with summary and help
	b.WriteString("\n")

	// Create a status bar style footer
	// [Time] [Help] [Status]

	timeStr := time.Now().Format("15:04:05")
	helpStr := "Quit: q / Ctrl+C"

	// Status summary
	var statusSummary string
	if len(m.services) > 0 {
		healthy := 0
		checking := 0
		for _, svc := range m.services {
			if svc.IsChecking {
				checking++
			} else if svc.Status == monitor.StatusHealthy {
				healthy++
			}
		}
		statusSummary = fmt.Sprintf("%d/%d Healthy", healthy, len(m.services))
	} else {
		statusSummary = "No services"
	}

	// Footer layout
	// 15:04:05 │ Quit: q / Ctrl+C                                         5/10 Healthy

	footerStyle := lipgloss.NewStyle().
		Foreground(colorMuted).
		BorderTop(true).
		BorderForeground(colorSubtle).
		Width(width).
		PaddingTop(1)

	left := fmt.Sprintf(" %s │ %s", timeStr, helpStr)
	right := fmt.Sprintf("%s ", statusSummary)

	gap := width - len(left) - len(right)
	if gap < 0 {
		gap = 0
	}

	footerContent := left + strings.Repeat(" ", gap) + right
	b.WriteString(footerStyle.Render(footerContent))
	b.WriteString("\n")

	return b.String()
}

// renderHeader renders an enhanced header with stats and visual appeal
func (m Model) renderHeader(width int, totalServices int) string {
	var b strings.Builder

	// Calculate stats
	healthy := 0
	unhealthy := 0
	checking := 0
	for _, svc := range m.services {
		if svc.IsChecking {
			checking++
		} else if svc.Status == monitor.StatusHealthy {
			healthy++
		} else {
			unhealthy++
		}
	}

	// Title
	title := "SCOUT"
	titleRendered := titleStyle.Render(title)

	// Stats
	var stats string
	if totalServices > 0 {
		healthyIndicator := healthyStyle.Render(fmt.Sprintf("● %d", healthy))
		unhealthyIndicator := unhealthyStyle.Render(fmt.Sprintf("● %d", unhealthy))
		checkingIndicator := checkingStyle.Render(fmt.Sprintf("● %d", checking))
		stats = fmt.Sprintf("%s  %s  %s", healthyIndicator, unhealthyIndicator, checkingIndicator)
	}

	// Layout: Title on left, Stats on right
	// SCOUT                                      ● 5  ● 0  ● 1

	availableWidth := width - lipgloss.Width(titleRendered) - lipgloss.Width(stats) - 2
	if availableWidth < 0 {
		availableWidth = 0
	}

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		titleRendered,
		strings.Repeat(" ", availableWidth),
		stats,
	)

	b.WriteString(header)
	b.WriteString("\n")

	// Gradient separator or just a line
	b.WriteString(lipgloss.NewStyle().Foreground(colorSubtle).Render(strings.Repeat("━", width)))

	return b.String()
}

// renderServiceGrid renders services in a grid layout
func (m Model) renderServiceGrid(services []ServiceState, cardWidth int, cols int) string {
	if cardWidth < 20 {
		cardWidth = 20
	}

	var rows []string
	for i := 0; i < len(services); i += cols {
		end := i + cols
		if end > len(services) {
			end = len(services)
		}

		var rowCards []string
		for j := i; j < end; j++ {
			card := m.renderServiceCompact(services[j], cardWidth)
			rowCards = append(rowCards, card)
		}

		// Join cards horizontally and add to rows
		row := lipgloss.JoinHorizontal(lipgloss.Top, rowCards...)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

// renderServiceCompact renders a service card for grid layout with modern design
func (m Model) renderServiceCompact(svc ServiceState, width int) string {
	var b strings.Builder

	// Determine border color based on status
	var borderColor lipgloss.Color
	switch svc.Status {
	case monitor.StatusHealthy:
		borderColor = colorHealthy
	case monitor.StatusUnhealthy:
		borderColor = colorUnhealthy
	case monitor.StatusChecking:
		borderColor = colorChecking
	default:
		borderColor = colorSubtle
	}

	// Status icon
	var statusIcon string
	if svc.IsChecking {
		if s, exists := m.spinners[svc.Name]; exists {
			statusIcon = s.View()
		} else {
			statusIcon = "⟳"
		}
	} else {
		statusIcon = m.getStatusIcon(svc.Status)
	}

	// Service name (truncate if needed)
	name := svc.Name
	maxNameLen := width - 6
	if len(name) > maxNameLen {
		name = name[:maxNameLen-1] + "…"
	}

	// Header: icon + name
	headerLine := fmt.Sprintf("%s %s", statusIcon, serviceNameStyle.Render(name))
	b.WriteString(headerLine)
	b.WriteString("\n")

	// Details section
	// Status code and response time on one line
	if (svc.StatusCode > 0 || svc.ResponseTime > 0) && !svc.IsChecking {
		var details []string
		if svc.StatusCode > 0 {
			codeStr := fmt.Sprintf("%d", svc.StatusCode)
			// Color code based on value
			var codeColor lipgloss.Color
			if svc.StatusCode >= 200 && svc.StatusCode < 300 {
				codeColor = colorHealthy
			} else if svc.StatusCode >= 300 && svc.StatusCode < 400 {
				codeColor = colorChecking
			} else {
				codeColor = colorUnhealthy
			}
			details = append(details, lipgloss.NewStyle().Foreground(codeColor).Bold(true).Render(codeStr))
		}
		if svc.ResponseTime > 0 {
			details = append(details, secondaryStyle.Render(m.formatDuration(svc.ResponseTime)))
		}

		// Join with a dot
		if len(details) > 0 {
			b.WriteString(strings.Join(details, secondaryStyle.Render(" • ")))
			b.WriteString("\n")
		}
	} else if svc.IsChecking {
		b.WriteString(secondaryStyle.Render("Checking..."))
		b.WriteString("\n")
	} else {
		b.WriteString(secondaryStyle.Render("Waiting..."))
		b.WriteString("\n")
	}

	// Last checked time (smaller)
	if !svc.LastChecked.IsZero() && !svc.IsChecking {
		b.WriteString(lipgloss.NewStyle().Foreground(colorSubtle).Render(m.formatTime(svc.LastChecked)))
	}

	// Error if present (truncate to fit)
	if svc.Error != nil {
		b.WriteString("\n")
		errMsg := svc.Error.Error()
		if len(errMsg) > width-4 {
			errMsg = errMsg[:width-7] + "…"
		}
		b.WriteString(errorStyle.Render(errMsg))
	}

	content := b.String()

	// Apply the dynamic border
	return baseCardStyle.
		Width(width).
		BorderForeground(borderColor).
		Render(content)
}

// getStatusIcon returns the icon for a status
func (m Model) getStatusIcon(status monitor.Status) string {
	switch status {
	case monitor.StatusHealthy:
		return "✓"
	case monitor.StatusUnhealthy:
		return "✗"
	case monitor.StatusChecking:
		return "●"
	default:
		return "?"
	}
}

// formatDuration formats a duration for display
func (m Model) formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// formatTime formats a time for display
func (m Model) formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return fmt.Sprintf("%d seconds ago", int(diff.Seconds()))
	}
	if diff < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
	}

	return t.Format("15:04:05")
}
