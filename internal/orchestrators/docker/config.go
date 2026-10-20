package docker

import (
	"github.com/Felipalds/rancher-tui/internal/core"
)

// DockerRancherConfig holds Docker Rancher-specific configuration
type DockerRancherConfig struct {
	RancherVersion    string
	RancherPrime      bool
	BootstrapPassword string
	ImageTag          string
	Debug             bool
	HostPort          string
	InstallTasks      string
}

// FromMap populates DockerRancherConfig from a map
func (c *DockerRancherConfig) FromMap(m map[string]interface{}) {
	if v, ok := m["rancher_version"].(string); ok {
		c.RancherVersion = v
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
	if v, ok := m["host_port"].(string); ok {
		c.HostPort = v
	}

	// Apply defaults
	if c.RancherVersion == "" {
		c.RancherVersion = "2.11.7"
	}
	if c.BootstrapPassword == "" {
		c.BootstrapPassword = "admin"
	}
	if c.HostPort == "" {
		c.HostPort = "443"
	}
}

// GetRequiredFields returns the form fields for Docker Rancher configuration
func GetRequiredFields() []core.FormField {
	return []core.FormField{
		{
			Name:        "rancher_version",
			Label:       "Rancher Version",
			Description: "Version of Rancher to deploy via Docker",
			Required:    false,
			Default:     "2.11.7",
			Type:        "string",
		},
		{
			Name:        "deploy_rancher",
			Label:       "Deploy Rancher",
			Description: "Always true for Docker install",
			Required:    false,
			Default:     true,
			Type:        "bool",
		},
	}
}

// GetDefaultConfig returns default Docker Rancher configuration
func GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"rancher_version": "2.11.7",
		"deploy_rancher":  true,
	}
}
