package k3s

import (
	"github.com/Felipalds/rancher-tui/internal/core"
)

// K3sConfig represents the configuration for K3s orchestrator
type K3sConfig struct {
	K3sVersion        string
	RancherVersion    string
	DeployRancher     bool
	RancherPrime      bool
	BootstrapPassword string
	ImageTag          string
	Debug             bool
	InitTasks         string
	JoinTasks         string
	AddonTasks        string
}

// NewDefaultK3sConfig returns a K3sConfig with sensible defaults
func NewDefaultK3sConfig() *K3sConfig {
	return &K3sConfig{
		K3sVersion:        "v1.30.3+k3s1",
		RancherVersion:    "2.10.2",
		DeployRancher:     true,
		BootstrapPassword: "admin",
	}
}

// GetRequiredFields returns the form fields required by the K3s orchestrator
func GetRequiredFields() []core.FormField {
	return []core.FormField{
		{
			Name:    "k3s_version",
			Label:   "K3s Version",
			Default: "v1.30.3+k3s1",
			Type:    "string",
		},
		{
			Name:    "rancher_version",
			Label:   "Rancher Version",
			Default: "2.10.2",
			Type:    "string",
		},
		{
			Name:    "deploy_rancher",
			Label:   "Deploy Rancher",
			Default: true,
			Type:    "bool",
		},
	}
}

// FromMap creates a K3sConfig from a map of configuration values
func FromMap(configMap map[string]interface{}) *K3sConfig {
	cfg := NewDefaultK3sConfig()

	if v, ok := configMap["k3s_version"].(string); ok && v != "" {
		cfg.K3sVersion = v
	}
	if v, ok := configMap["rancher_version"].(string); ok && v != "" {
		cfg.RancherVersion = v
	}
	if v, ok := configMap["deploy_rancher"].(bool); ok {
		cfg.DeployRancher = v
	}
	if v, ok := configMap["rancher_prime"].(bool); ok {
		cfg.RancherPrime = v
	}
	if v, ok := configMap["rancher_bootstrap_password"].(string); ok && v != "" {
		cfg.BootstrapPassword = v
	}
	if v, ok := configMap["rancher_image_tag"].(string); ok {
		cfg.ImageTag = v
	}
	if v, ok := configMap["rancher_debug"].(bool); ok {
		cfg.Debug = v
	}

	return cfg
}
