package harvester

import (
	"fmt"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// Supported values for the Harvester install target and local VM backend.
const (
	TargetCloud   = "cloud"
	TargetVagrant = "vagrant"

	BackendLibvirt = "libvirt"

	defaultHarvesterVersion = "v1.4.0"
	defaultNodeCount        = 1
)

// HarvesterConfig holds Harvester HCI configuration.
//
// Phase 1 only scaffolds this: the config, validation and form fields are real,
// but playbook/inventory generation is not implemented yet.
type HarvesterConfig struct {
	InstallTarget    string // "cloud" | "vagrant"
	VagrantBackend   string // "libvirt" (KVM) — only backend supported in Phase 1
	HarvesterVersion string
	NodeCount        int
}

// FromMap populates the config from a generic map, applying defaults.
func (c *HarvesterConfig) FromMap(m map[string]interface{}) {
	if v, ok := m["install_target"].(string); ok {
		c.InstallTarget = v
	}
	if v, ok := m["vagrant_backend"].(string); ok {
		c.VagrantBackend = v
	}
	if v, ok := m["harvester_version"].(string); ok {
		c.HarvesterVersion = v
	}
	if v, ok := m["node_count"].(int); ok {
		c.NodeCount = v
	}

	// Apply defaults
	if c.InstallTarget == "" {
		c.InstallTarget = TargetVagrant
	}
	if c.VagrantBackend == "" {
		c.VagrantBackend = BackendLibvirt
	}
	if c.HarvesterVersion == "" {
		c.HarvesterVersion = defaultHarvesterVersion
	}
	if c.NodeCount == 0 {
		c.NodeCount = defaultNodeCount
	}
}

// Validate checks the Harvester configuration.
func (c *HarvesterConfig) Validate() error {
	switch c.InstallTarget {
	case TargetCloud, TargetVagrant:
	default:
		return fmt.Errorf("invalid install_target %q (must be %q or %q)", c.InstallTarget, TargetCloud, TargetVagrant)
	}

	if c.InstallTarget == TargetVagrant && c.VagrantBackend != BackendLibvirt {
		return fmt.Errorf("invalid vagrant_backend %q (only %q is supported in Phase 1)", c.VagrantBackend, BackendLibvirt)
	}

	if c.NodeCount < 1 {
		return fmt.Errorf("node_count must be at least 1, got %d", c.NodeCount)
	}
	return nil
}

// GetRequiredFields returns the TUI form fields for Harvester configuration.
func GetRequiredFields() []core.FormField {
	return []core.FormField{
		{
			Name:        "install_target",
			Label:       "Install Target",
			Description: "Where to install Harvester",
			Required:    true,
			Default:     TargetVagrant,
			Type:        "select",
			Options:     []string{TargetVagrant, TargetCloud},
		},
		{
			Name:        "vagrant_backend",
			Label:       "Vagrant Backend",
			Description: "Local VM backend (Vagrant installs only)",
			Required:    false,
			Default:     BackendLibvirt,
			Type:        "select",
			Options:     []string{BackendLibvirt},
		},
		{
			Name:        "harvester_version",
			Label:       "Harvester Version",
			Description: "Version of Harvester to install",
			Required:    false,
			Default:     defaultHarvesterVersion,
			Type:        "string",
		},
		{
			Name:        "node_count",
			Label:       "Node Count",
			Description: "Number of Harvester nodes",
			Required:    false,
			Default:     defaultNodeCount,
			Type:        "int",
		},
	}
}

// GetDefaultConfig returns default Harvester configuration values.
func GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"install_target":    TargetVagrant,
		"vagrant_backend":   BackendLibvirt,
		"harvester_version": defaultHarvesterVersion,
		"node_count":        defaultNodeCount,
	}
}
