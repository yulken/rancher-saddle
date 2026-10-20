package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// ExpandPath expands ~ to home directory and resolves relative paths
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Expand tilde to home directory
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
	} else if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = homeDir
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absPath, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate provider selection
	if c.Provider == "" {
		return fmt.Errorf("provider must be specified")
	}

	// Validate orchestrator selection
	if c.Orchestrator == "" {
		return fmt.Errorf("orchestrator must be specified")
	}

	// Validate common fields
	if c.InstanceCount < 1 {
		return fmt.Errorf("instance_count must be at least 1")
	}

	if c.SSHKeyName == "" {
		return fmt.Errorf("ssh_key_name is required")
	}

	if c.SSHPrivateKeyPath == "" {
		return fmt.Errorf("ssh_private_key_path is required")
	}

	// Expand path (handle ~ and relative paths)
	expandedPath, err := ExpandPath(c.SSHPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand ssh_private_key_path: %w", err)
	}

	// Check if SSH private key file exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return fmt.Errorf("ssh_private_key_path '%s' (expanded to '%s') does not exist", c.SSHPrivateKeyPath, expandedPath)
	}

	// Update the config with expanded path for later use
	c.SSHPrivateKeyPath = expandedPath

	return nil
}

// ValidateWithRegistry validates the configuration against registered providers and orchestrators
func (c *Config) ValidateWithRegistry(registry *core.Registry) error {
	// Basic validation
	if err := c.Validate(); err != nil {
		return err
	}

	// Get and validate provider
	provider, err := registry.GetProvider(c.GetProviderType())
	if err != nil {
		return fmt.Errorf("invalid provider: %w", err)
	}

	if err := provider.Validate(c.ProviderConfig); err != nil {
		return fmt.Errorf("provider validation failed: %w", err)
	}

	// Get and validate orchestrator
	orchestrator, err := registry.GetOrchestrator(c.GetOrchestratorType())
	if err != nil {
		return fmt.Errorf("invalid orchestrator: %w", err)
	}

	if err := orchestrator.Validate(c.OrchestratorConfig); err != nil {
		return fmt.Errorf("orchestrator validation failed: %w", err)
	}

	return nil
}
