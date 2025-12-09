package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultCheckInterval = "30s"
	DefaultTimeout       = "5s"
	DefaultRetryAttempts = 3
)

// Config represents the scout configuration
type Config struct {
	CheckInterval string    `yaml:"check_interval"`
	Timeout       string    `yaml:"timeout"`
	RetryAttempts int       `yaml:"retry_attempts"`
	Services      []Service `yaml:"services"`
}

// Auth represents authentication configuration for a service
type Auth struct {
	Type     string `yaml:"type,omitempty"` // "bearer", "basic", or empty
	Token    string `yaml:"token,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// JSONAssertion represents a JSON path assertion
type JSONAssertion struct {
	Path     string      `yaml:"path"`     // JSON path (e.g., "status.database" or "data[0].healthy")
	Value    interface{} `yaml:"value"`    // Expected value to match
	Operator string      `yaml:"operator"` // "==", "!=", ">", "<", ">=", "<=", "contains"
}

// Service represents a service to monitor
type Service struct {
	Name           string            `yaml:"name"`
	URL            string            `yaml:"url"`
	HealthEndpoint string            `yaml:"health_endpoint,omitempty"`
	Method         string            `yaml:"method,omitempty"`
	ExpectedStatus int               `yaml:"expected_status,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty"`
	Type           string            `yaml:"type,omitempty"`
	Auth           *Auth             `yaml:"auth,omitempty"`
	JSONAssertions []JSONAssertion   `yaml:"json_assertions,omitempty"`
}

// GetConfigPath returns the path to the global config file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", "scout", "config.yml"), nil
}

// InitConfig creates the config directory and file with default content
func InitConfig(force bool) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
	}

	// Create the directory
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write default config
	defaultConfig := getDefaultConfig()
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadConfig reads and parses the config file
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig writes the config back to the file
func SaveConfig(cfg *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// AddService adds a new service to the config
func (c *Config) AddService(service Service) error {
	// Check for duplicate names
	for _, s := range c.Services {
		if s.Name == service.Name {
			return fmt.Errorf("service with name '%s' already exists", service.Name)
		}
	}

	c.Services = append(c.Services, service)
	return nil
}

// RemoveService removes a service by name from the config
func (c *Config) RemoveService(name string) error {
	for i, s := range c.Services {
		if s.Name == name {
			c.Services = append(c.Services[:i], c.Services[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("service '%s' not found", name)
}

// getDefaultConfig returns the default configuration as YAML
func getDefaultConfig() string {
	return fmt.Sprintf(`# Scout Configuration
# Global defaults for all services
check_interval: %s
timeout: %s
retry_attempts: %d

# Define your services below
services:
  - name: example-api
    url: https://api.example.com
    health_endpoint: /health
    method: GET
    expected_status: 200
`, DefaultCheckInterval, DefaultTimeout, DefaultRetryAttempts)
}

// ResolveEnv replaces environment variable placeholders with actual values
// Supports ${VAR_NAME} syntax
func ResolveEnv(value string) string {
	return os.ExpandEnv(strings.NewReplacer(
		"${", "$",
		"}", "",
	).Replace(value))
}
