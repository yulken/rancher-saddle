package harvester

import (
	"fmt"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// Supported VIP modes.
const (
	VIPModeDHCP   = "DHCP"
	VIPModeStatic = "static"
)

// Defaults for a local Harvester cluster.
const (
	defaultVersion      = "v1.4.1"
	defaultClusterToken = "token"
	defaultPassword     = "p@ssword"
	// Static VIP outside the libvirt DHCP range (.100-.200) so it is
	// deterministic and pollable. DHCP mode would assign an unknown address.
	defaultVIP     = "192.168.0.50"
	defaultVIPMode = VIPModeStatic
	// enp1s0 is the usual name for a single virtio NIC on a q35 machine (the
	// layout we generate). Adjust in the form if your guest names it differently.
	defaultMgmtInterface = "enp1s0"
	defaultNTPServer     = "0.suse.pool.ntp.org"
)

// HarvesterConfig holds Harvester HCI install configuration.
type HarvesterConfig struct {
	Version       string
	ClusterToken  string
	Password      string
	VIP           string
	VIPMode       string
	MgmtInterface string
	NTPServers    []string
	DeployRancher bool
	NodeCount     int
}

// FromMap populates the config from a generic map, applying defaults.
func (c *HarvesterConfig) FromMap(m map[string]interface{}) {
	if v, ok := m["harvester_version"].(string); ok && v != "" {
		c.Version = v
	} else if v, ok := m["version"].(string); ok && v != "" {
		c.Version = v
	}
	if v, ok := m["cluster_token"].(string); ok {
		c.ClusterToken = v
	}
	if v, ok := m["password"].(string); ok {
		c.Password = v
	}
	if v, ok := m["vip"].(string); ok {
		c.VIP = v
	}
	if v, ok := m["vip_mode"].(string); ok {
		c.VIPMode = v
	}
	if v, ok := m["mgmt_interface"].(string); ok {
		c.MgmtInterface = v
	}
	if ntp := parseNTP(m["ntp_servers"]); ntp != nil {
		c.NTPServers = ntp
	}
	c.DeployRancher = boolFrom(m, "deploy_rancher", c.DeployRancher)
	c.NodeCount = intFrom(m, "node_count", c.NodeCount)
	if c.NodeCount == 0 {
		c.NodeCount = intFrom(m, "instance_count", 0)
	}

	c.applyDefaults()
}

func (c *HarvesterConfig) applyDefaults() {
	if c.Version == "" {
		c.Version = defaultVersion
	}
	if c.ClusterToken == "" {
		c.ClusterToken = defaultClusterToken
	}
	if c.Password == "" {
		c.Password = defaultPassword
	}
	if c.VIP == "" {
		c.VIP = defaultVIP
	}
	if c.VIPMode == "" {
		c.VIPMode = defaultVIPMode
	}
	if c.MgmtInterface == "" {
		c.MgmtInterface = defaultMgmtInterface
	}
	if len(c.NTPServers) == 0 {
		c.NTPServers = []string{defaultNTPServer}
	}
	if c.NodeCount == 0 {
		c.NodeCount = 1
	}
}

// Validate checks the Harvester configuration.
func (c *HarvesterConfig) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("harvester_version is required")
	}
	if c.VIP == "" {
		return fmt.Errorf("vip is required")
	}
	switch c.VIPMode {
	case VIPModeDHCP, VIPModeStatic:
	default:
		return fmt.Errorf("invalid vip_mode %q (must be %q or %q)", c.VIPMode, VIPModeDHCP, VIPModeStatic)
	}
	if c.NodeCount < 1 {
		return fmt.Errorf("node_count must be at least 1, got %d", c.NodeCount)
	}
	return nil
}

// parseNTP accepts a []string, []interface{}, or a comma/space separated string.
func parseNTP(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		fields := strings.FieldsFunc(t, func(r rune) bool { return r == ',' || r == ' ' })
		if len(fields) == 0 {
			return nil
		}
		return fields
	default:
		return nil
	}
}

func boolFrom(m map[string]interface{}, key string, fallback bool) bool {
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		return v == "Yes" || v == "true" || v == "yes"
	default:
		return fallback
	}
}

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

// GetRequiredFields returns the TUI form fields for Harvester configuration.
func GetRequiredFields() []core.FormField {
	return []core.FormField{
		{Name: "harvester_version", Label: "Harvester Version", Description: "Version of Harvester to install", Default: defaultVersion, Type: "string"},
		{Name: "cluster_token", Label: "Cluster Token", Description: "Shared token used by nodes to join", Default: defaultClusterToken, Type: "string"},
		{Name: "password", Label: "Node Password", Description: "Password for the rancher OS user", Default: defaultPassword, Type: "string"},
		{Name: "vip", Label: "Cluster VIP", Description: "Virtual IP for the Harvester cluster", Default: defaultVIP, Type: "string"},
		{Name: "vip_mode", Label: "VIP Mode", Description: "How the VIP is assigned", Default: defaultVIPMode, Type: "select", Options: []string{VIPModeDHCP, VIPModeStatic}},
		{Name: "ntp_servers", Label: "NTP Servers", Description: "Comma-separated NTP servers", Default: defaultNTPServer, Type: "string"},
		{Name: "deploy_rancher", Label: "Deploy Rancher", Description: "Deploy Rancher alongside Harvester", Default: "No", Type: "select", Options: []string{"No", "Yes"}},
	}
}

// GetDefaultConfig returns default Harvester configuration values.
func GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"harvester_version": defaultVersion,
		"cluster_token":     defaultClusterToken,
		"password":          defaultPassword,
		"vip":               defaultVIP,
		"vip_mode":          defaultVIPMode,
		"ntp_servers":       defaultNTPServer,
		"deploy_rancher":    false,
	}
}
