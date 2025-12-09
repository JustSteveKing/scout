package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/juststeveking/scout/internal/monitor"
)

// Model represents the TUI application state
type Model struct {
	services        []ServiceState
	width           int
	height          int
	lastUpdate      time.Time
	quitting        bool
	monitor         *monitor.Monitor
	monitorCancel   func()
	spinners        map[string]spinner.Model
	selectedIndex   int
	showDetail      bool
	detailName      string
	showErrorDetail bool
	errorDetailName string
	clipboardMsg    string
	clipboardTime   time.Time
	pausedServices  map[string]bool

	// Form state
	form     *huh.Form
	showForm bool
	formData *FormData
}

// FormData holds the data for the add service form
type FormData struct {
	Name           string
	URL            string
	Method         string
	ExpectedStatus string
	HealthEndpoint string
	AuthType       string
	AuthToken      string
	AuthUsername   string
	AuthPassword   string
	Headers        string // Formatted as key:value,key:value
	JSONAssertions string // Formatted as path:value:operator,path:value:operator
}

// ServiceState tracks the current state of a service
type ServiceState struct {
	Name         string
	Status       monitor.Status
	ResponseTime time.Duration
	Message      string
	LastChecked  time.Time
	StatusCode   int
	Error        error
	IsChecking   bool
	Checks       []string
	Paused       bool
}

// NewModel creates a new TUI model
func NewModel(m *monitor.Monitor, cancel func()) Model {
	return Model{
		services:       make([]ServiceState, 0),
		monitor:        m,
		monitorCancel:  cancel,
		lastUpdate:     time.Now(),
		spinners:       make(map[string]spinner.Model),
		selectedIndex:  0,
		pausedServices: make(map[string]bool),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		waitForResults(m.monitor),
		tea.EnterAltScreen,
		doTick(),
	)
}

// resultMsg wraps a monitor result for Bubble Tea
type resultMsg monitor.Result

// waitForResults listens for monitor results
func waitForResults(mon *monitor.Monitor) tea.Cmd {
	return func() tea.Msg {
		result := <-mon.Results()
		return resultMsg(result)
	}
}

// tickMsg is sent on every tick
type tickMsg time.Time

// doTick returns a command that waits for the next tick
func doTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// clipboardMsg is sent when clipboard operation completes
type clipboardMsg struct {
	success bool
	message string
}
