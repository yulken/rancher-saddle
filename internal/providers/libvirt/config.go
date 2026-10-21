package libvirt

import (
	"fmt"
	"path/filepath"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// BackendLibvirt is the only VM backend supported (KVM via libvirt).
const BackendLibvirt = "libvirt"

// Defaults for a local Harvester lab. These mirror the sizing used by
// harvester/ipxe-examples (vagrant-pxe-harvester).
const (
	defaultNodeCount        = 3
	defaultNodeCPU          = 8
	defaultNodeMemoryMB     = 16384
	defaultNodeDiskGB       = 250
	defaultNetworkSubnet    = "192.168.0.0/24"
	defaultNodePrefix       = "harvester-node"
	defaultHarvesterVersion = "v1.4.1"
	defaultFirmware         = FirmwareUEFI
)

// Supported VM firmware modes.
const (
	FirmwareUEFI = "uefi"
	FirmwareBIOS = "bios"
)

// Config holds configuration for local VMs provisioned via Vagrant +
// libvirt for a Harvester PXE install.
type Config struct {
	ClusterName      string // required — names the libvirt network and VMs
	NodeCount        int    // number of Harvester node VMs
	NodeCPU          int    // vCPUs per Harvester node
	NodeMemoryMB     int    // memory per Harvester node, in MB
	NodeDiskGB       int    // data disk per Harvester node, in GB
	NetworkSubnet    string // libvirt network subnet, e.g. "192.168.0.0/24"
	NodePrefix       string // hostname prefix for Harvester nodes
	HarvesterVersion string // media version, used for the ISO synced-folder path
	ISOPath          string // optional: path to a specific local Harvester .iso
	Firmware         string // "uefi" (default) or "bios"
}

// FromMap populates the config from a generic map, applying defaults.
func (c *Config) FromMap(m map[string]interface{}) {
	if v, ok := m["cluster_name"].(string); ok {
		c.ClusterName = v
	}
	c.NodeCount = intFrom(m, "node_count", c.NodeCount)
	// Fall back to the common instance_count when node_count is not set.
	if c.NodeCount == 0 {
		c.NodeCount = intFrom(m, "instance_count", 0)
	}
	c.NodeCPU = intFrom(m, "node_cpu", c.NodeCPU)
	c.NodeMemoryMB = intFrom(m, "node_memory_mb", c.NodeMemoryMB)
	c.NodeDiskGB = intFrom(m, "node_disk_gb", c.NodeDiskGB)
	if v, ok := m["network_subnet"].(string); ok {
		c.NetworkSubnet = v
	}
	if v, ok := m["node_prefix"].(string); ok && v != "" {
		c.NodePrefix = v
	}
	if v, ok := m["harvester_version"].(string); ok {
		c.HarvesterVersion = v
	}
	if v, ok := m["harvester_iso_path"].(string); ok {
		c.ISOPath = v
	}
	if v, ok := m["firmware"].(string); ok {
		c.Firmware = v
	}

	c.applyDefaults()
}

// applyDefaults fills zero-valued fields with their defaults.
func (c *Config) applyDefaults() {
	if c.NodeCount == 0 {
		c.NodeCount = defaultNodeCount
	}
	if c.NodeCPU == 0 {
		c.NodeCPU = defaultNodeCPU
	}
	if c.NodeMemoryMB == 0 {
		c.NodeMemoryMB = defaultNodeMemoryMB
	}
	if c.NodeDiskGB == 0 {
		c.NodeDiskGB = defaultNodeDiskGB
	}
	if c.NetworkSubnet == "" {
		c.NetworkSubnet = defaultNetworkSubnet
	}
	if c.NodePrefix == "" {
		c.NodePrefix = defaultNodePrefix
	}
	if c.HarvesterVersion == "" {
		c.HarvesterVersion = defaultHarvesterVersion
	}
	if c.Firmware == "" {
		c.Firmware = defaultFirmware
	}
}

// intFrom reads an int from a map, tolerating int and float64 (JSON) encodings.
func intFrom(m map[string]interface{}, key string, fallback int) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return fallback
	}
}

// MediaDir returns the directory the Harvester media is served from: the parent
// of a user-supplied ISO when set, otherwise the shared version cache.
func (c *Config) MediaDir(projectRoot string) string {
	if c.ISOPath != "" {
		return filepath.Dir(c.ISOPath)
	}
	return CacheDir(projectRoot, c.HarvesterVersion)
}

// Validate checks the libvirt VM configuration.
func (c *Config) Validate() error {
	if c.ClusterName == "" {
		return fmt.Errorf("cluster_name is required")
	}
	if c.NodeCount < 1 {
		return fmt.Errorf("node_count must be at least 1, got %d", c.NodeCount)
	}
	if c.NodeCPU < 1 {
		return fmt.Errorf("node_cpu must be at least 1, got %d", c.NodeCPU)
	}
	if c.NodeMemoryMB < 1 {
		return fmt.Errorf("node_memory_mb must be at least 1, got %d", c.NodeMemoryMB)
	}
	if c.NodeDiskGB < 1 {
		return fmt.Errorf("node_disk_gb must be at least 1, got %d", c.NodeDiskGB)
	}
	if _, err := subnetBase(c.NetworkSubnet); err != nil {
		return err
	}
	switch c.Firmware {
	case FirmwareUEFI, FirmwareBIOS:
	default:
		return fmt.Errorf("invalid firmware %q (must be %q or %q)", c.Firmware, FirmwareUEFI, FirmwareBIOS)
	}
	return nil
}

// GetRequiredFields returns the TUI form fields for libvirt VM configuration.
func GetRequiredFields() []core.FormField {
	return []core.FormField{
		{
			Name:        "node_count",
			Label:       "Number of Harvester Nodes",
			Description: "How many Harvester node VMs to create",
			Required:    false,
			Default:     defaultNodeCount,
			Type:        "int",
		},
		{
			Name:        "node_cpu",
			Label:       "CPUs per Node",
			Description: "vCPUs per Harvester node",
			Required:    false,
			Default:     defaultNodeCPU,
			Type:        "int",
		},
		{
			Name:        "node_memory_mb",
			Label:       "Memory per Node (MB)",
			Description: "Memory per Harvester node in megabytes",
			Required:    false,
			Default:     defaultNodeMemoryMB,
			Type:        "int",
		},
		{
			Name:        "node_disk_gb",
			Label:       "Disk per Node (GB)",
			Description: "Data disk per Harvester node in gigabytes",
			Required:    false,
			Default:     defaultNodeDiskGB,
			Type:        "int",
		},
		{
			Name:        "network_subnet",
			Label:       "Network Subnet",
			Description: "Isolated libvirt network subnet (/24)",
			Required:    false,
			Default:     defaultNetworkSubnet,
			Type:        "string",
		},
		{
			Name:        "node_prefix",
			Label:       "Node Hostname Prefix",
			Description: "Prefix for Harvester node hostnames",
			Required:    false,
			Default:     defaultNodePrefix,
			Type:        "string",
		},
		{
			Name:        "firmware",
			Label:       "Firmware",
			Description: "VM firmware (UEFI needs OVMF/edk2 installed)",
			Required:    false,
			Default:     defaultFirmware,
			Type:        "select",
			Options:     []string{FirmwareUEFI, FirmwareBIOS},
		},
	}
}

// GetDefaultConfig returns default libvirt VM configuration values.
func GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"node_count":     defaultNodeCount,
		"node_cpu":       defaultNodeCPU,
		"node_memory_mb": defaultNodeMemoryMB,
		"node_disk_gb":   defaultNodeDiskGB,
		"network_subnet": defaultNetworkSubnet,
		"node_prefix":    defaultNodePrefix,
		"firmware":       defaultFirmware,
	}
}
