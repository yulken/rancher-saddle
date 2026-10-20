package config

import (
	"encoding/json"
	"os"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// Config holds the dynamic application configuration for deploying Kubernetes clusters
type Config struct {
	// Provider and Orchestrator selection
	Provider     string `json:"provider"`     // "aws", "azure", "gcp", "vsphere"
	Orchestrator string `json:"orchestrator"` // "rke2", "k3s", "minikube", "kubeadm"

	// Common cluster settings
	ClusterName       string `json:"cluster_name"`
	NodePrefix        string `json:"node_prefix"`
	InstanceCount     int    `json:"instance_count"`
	SSHKeyName        string `json:"ssh_key_name"`
	SSHPrivateKeyPath string `json:"ssh_private_key_path"`
	SSHUser           string `json:"ssh_user"`

	// Dynamic provider-specific configuration
	ProviderConfig map[string]interface{} `json:"provider_config"`

	// Dynamic orchestrator-specific configuration
	OrchestratorConfig map[string]interface{} `json:"orchestrator_config"`

	// Add-ons (optional features like Rancher, Longhorn, etc.)
	Addons []string `json:"addons"`
}

// LoadConfig reads the configuration from the specified JSON file
func LoadConfig(path string) (*Config, error) {
	// If file doesn't exist, return default config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return GetDefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply defaults for empty fields
	applyDefaults(&cfg)

	return &cfg, nil
}

// Save writes the configuration to the specified JSON file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// File permission 0600 because it may contain secrets
	return os.WriteFile(path, data, 0600)
}

// GetDefaultConfig returns a configuration with sensible defaults
func GetDefaultConfig() *Config {
	return &Config{
		Provider:           string(core.ProviderAWS),
		Orchestrator:       string(core.OrchestratorRKE2),
		ClusterName:        "",
		NodePrefix:         "k8s-node",
		InstanceCount:      1,
		SSHKeyName:         "",
		SSHPrivateKeyPath:  "",
		SSHUser:            "ubuntu",
		ProviderConfig:     make(map[string]interface{}),
		OrchestratorConfig: make(map[string]interface{}),
		Addons:             []string{},
	}
}

// applyDefaults sets default values for empty configuration fields
func applyDefaults(cfg *Config) {
	if cfg.Provider == "" {
		cfg.Provider = string(core.ProviderAWS)
	}
	if cfg.Orchestrator == "" {
		cfg.Orchestrator = string(core.OrchestratorRKE2)
	}
	if cfg.NodePrefix == "" {
		cfg.NodePrefix = "k8s-node"
	}
	if cfg.InstanceCount == 0 {
		cfg.InstanceCount = 1
	}
	if cfg.SSHUser == "" {
		cfg.SSHUser = "ubuntu"
	}
	if cfg.ProviderConfig == nil {
		cfg.ProviderConfig = make(map[string]interface{})
	}
	if cfg.OrchestratorConfig == nil {
		cfg.OrchestratorConfig = make(map[string]interface{})
	}
	if cfg.Addons == nil {
		cfg.Addons = []string{}
	}
}

// GetProviderType returns the provider type as a core.ProviderType
func (c *Config) GetProviderType() core.ProviderType {
	return core.ProviderType(c.Provider)
}

// GetOrchestratorType returns the orchestrator type as a core.OrchestratorType
func (c *Config) GetOrchestratorType() core.OrchestratorType {
	return core.OrchestratorType(c.Orchestrator)
}
