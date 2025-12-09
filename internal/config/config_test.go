package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigOperations(t *testing.T) {
	// Setup temp home dir
	tmpHome, err := os.MkdirTemp("", "scout-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Set HOME to temp dir
	os.Setenv("HOME", tmpHome)

	// Test InitConfig
	err = InitConfig(false)
	if err != nil {
		t.Errorf("InitConfig failed: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpHome, ".config", "scout", "config.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Test LoadConfig
	cfg, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("Config is nil")
	}

	// Test AddService
	newService := Service{
		Name: "test-service",
		URL:  "http://example.com",
	}
	err = cfg.AddService(newService)
	if err != nil {
		t.Errorf("AddService failed: %v", err)
	}

	if len(cfg.Services) != 2 { // Default config has 1 service
		t.Errorf("Expected 2 services, got %d", len(cfg.Services))
	}

	// Test SaveConfig
	err = SaveConfig(cfg)
	if err != nil {
		t.Errorf("SaveConfig failed: %v", err)
	}

	// Reload and verify
	cfg2, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig failed: %v", err)
	}
	if len(cfg2.Services) != 2 {
		t.Errorf("Expected 2 services after reload, got %d", len(cfg2.Services))
	}

	// Test RemoveService
	err = cfg.RemoveService("test-service")
	if err != nil {
		t.Errorf("RemoveService failed: %v", err)
	}
	if len(cfg.Services) != 1 {
		t.Errorf("Expected 1 service after remove, got %d", len(cfg.Services))
	}
}

func TestResolveEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "world")
	defer os.Unsetenv("TEST_VAR")

	val := ResolveEnv("hello ${TEST_VAR}")
	if val != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", val)
	}
}
