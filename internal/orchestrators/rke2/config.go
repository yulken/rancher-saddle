package rke2

import (
	"github.com/Felipalds/rancher-tui/internal/core"
)

// RKE2Config holds RKE2-specific configuration
type RKE2Config struct {
	RKE2Version       string
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

// FromMap creates RKE2Config from a map
func (c *RKE2Config) FromMap(m map[string]interface{}) {
	if v, ok := m["rke2_version"].(string); ok {
		c.RKE2Version = v
	}
	if v, ok := m["rancher_version"].(string); ok {
		c.RancherVersion = v
	}
	if v, ok := m["deploy_rancher"].(bool); ok {
		c.DeployRancher = v
	}
	if v, ok := m["rancher_prime"].(bool); ok {
		c.RancherPrime = v
	}
	if v, ok := m["rancher_bootstrap_password"].(string); ok {
		c.BootstrapPassword = v
	}
	if v, ok := m["rancher_image_tag"].(string); ok {
		c.ImageTag = v
	}
	if v, ok := m["rancher_debug"].(bool); ok {
		c.Debug = v
	}

	// Apply defaults
	if c.RKE2Version == "" {
		c.RKE2Version = "v1.33.7+rke2r1"
	}
	if c.RancherVersion == "" {
		c.RancherVersion = "2.10.2"
	}
	if c.BootstrapPassword == "" {
		c.BootstrapPassword = "admin"
	}
	// Default to deploying Rancher if not explicitly set
	if _, ok := m["deploy_rancher"]; !ok {
		c.DeployRancher = true
	}
}

// GetRequiredFields returns the form fields for RKE2 configuration
func GetRequiredFields() []core.FormField {
	return []core.FormField{
		{
			Name:        "rke2_version",
			Label:       "RKE2 Version",
			Description: "Version of RKE2 to install",
			Required:    false,
			Default:     "v1.33.7+rke2r1",
			Type:        "string",
		},
		{
			Name:        "rancher_version",
			Label:       "Rancher Version",
			Description: "Version of Rancher to deploy",
			Required:    false,
			Default:     "2.10.2",
			Type:        "string",
		},
		{
			Name:        "deploy_rancher",
			Label:       "Deploy Rancher",
			Description: "Whether to deploy Rancher management server",
			Required:    false,
			Default:     true,
			Type:        "bool",
		},
	}
}

// GetDefaultConfig returns default RKE2 configuration
func GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"rke2_version":    "v1.33.7+rke2r1",
		"rancher_version": "2.10.2",
		"deploy_rancher":  true,
	}
}
