package vagrant

import (
	"fmt"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// BackendLibvirt is the only VM backend supported in Phase 1 (KVM via libvirt).
const BackendLibvirt = "libvirt"

const (
	defaultMemory = 8192 // MB
	defaultCPUs   = 4
	defaultBox    = "opensuse/Leap-15.6.x86_64"
)

// VagrantConfig holds configuration for local VMs provisioned via Vagrant.
//
// Phase 1 scaffolds this: config, validation and form fields are real, but
// infrastructure generation is not implemented yet.
type VagrantConfig struct {
	Backend string // "libvirt" (KVM)
	Memory  int    // MB per VM
	CPUs    int    // vCPUs per VM
	Box     string // Vagrant box image
}

// FromMap populates the config from a generic map, applying defaults.
func (c *VagrantConfig) FromMap(m map[string]interface{}) {
	if v, ok := m["backend"].(string); ok {
		c.Backend = v
	}
	if v, ok := m["memory"].(int); ok {
		c.Memory = v
	}
	if v, ok := m["cpus"].(int); ok {
		c.CPUs = v
	}
	if v, ok := m["box"].(string); ok {
		c.Box = v
	}

	if c.Backend == "" {
		c.Backend = BackendLibvirt
	}
	if c.Memory == 0 {
		c.Memory = defaultMemory
	}
	if c.CPUs == 0 {
		c.CPUs = defaultCPUs
	}
	if c.Box == "" {
		c.Box = defaultBox
	}
}

// Validate checks the Vagrant configuration.
func (c *VagrantConfig) Validate() error {
	if c.Backend != BackendLibvirt {
		return fmt.Errorf("invalid backend %q (only %q is supported in Phase 1)", c.Backend, BackendLibvirt)
	}
	if c.Memory < 1 {
		return fmt.Errorf("memory must be at least 1 MB, got %d", c.Memory)
	}
	if c.CPUs < 1 {
		return fmt.Errorf("cpus must be at least 1, got %d", c.CPUs)
	}
	return nil
}

// GetRequiredFields returns the TUI form fields for Vagrant configuration.
func GetRequiredFields() []core.FormField {
	return []core.FormField{
		{
			Name:        "backend",
			Label:       "VM Backend",
			Description: "Local virtualization backend",
			Required:    true,
			Default:     BackendLibvirt,
			Type:        "select",
			Options:     []string{BackendLibvirt},
		},
		{
			Name:        "memory",
			Label:       "Memory (MB)",
			Description: "Memory per VM in megabytes",
			Required:    false,
			Default:     defaultMemory,
			Type:        "int",
		},
		{
			Name:        "cpus",
			Label:       "vCPUs",
			Description: "Virtual CPUs per VM",
			Required:    false,
			Default:     defaultCPUs,
			Type:        "int",
		},
		{
			Name:        "box",
			Label:       "Vagrant Box",
			Description: "Base box image for the VMs",
			Required:    false,
			Default:     defaultBox,
			Type:        "string",
		},
	}
}

// GetDefaultConfig returns default Vagrant configuration values.
func GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"backend": BackendLibvirt,
		"memory":  defaultMemory,
		"cpus":    defaultCPUs,
		"box":     defaultBox,
	}
}
