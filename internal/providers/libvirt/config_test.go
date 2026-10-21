package libvirt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_FromMap_Defaults(t *testing.T) {
	cfg := &Config{}
	cfg.FromMap(map[string]interface{}{"cluster_name": "lab"})

	assert.Equal(t, "lab", cfg.ClusterName)
	assert.Equal(t, defaultNodeCount, cfg.NodeCount)
	assert.Equal(t, defaultNodeCPU, cfg.NodeCPU)
	assert.Equal(t, defaultNodeMemoryMB, cfg.NodeMemoryMB)
	assert.Equal(t, defaultNodeDiskGB, cfg.NodeDiskGB)
	assert.Equal(t, defaultNetworkSubnet, cfg.NetworkSubnet)
	assert.Equal(t, defaultNodePrefix, cfg.NodePrefix)
	assert.Equal(t, defaultHarvesterVersion, cfg.HarvesterVersion)
}

func TestConfig_FromMap_Overrides(t *testing.T) {
	cfg := &Config{}
	cfg.FromMap(map[string]interface{}{
		"cluster_name":      "prod",
		"node_count":        5,
		"node_cpu":          4,
		"node_memory_mb":    8192,
		"node_disk_gb":      100,
		"network_subnet":    "10.0.0.0/24",
		"node_prefix":       "hv",
		"harvester_version": "v1.5.0",
	})

	assert.Equal(t, 5, cfg.NodeCount)
	assert.Equal(t, 4, cfg.NodeCPU)
	assert.Equal(t, 8192, cfg.NodeMemoryMB)
	assert.Equal(t, 100, cfg.NodeDiskGB)
	assert.Equal(t, "10.0.0.0/24", cfg.NetworkSubnet)
	assert.Equal(t, "hv", cfg.NodePrefix)
	assert.Equal(t, "v1.5.0", cfg.HarvesterVersion)
}

func TestConfig_FromMap_FallsBackToInstanceCount(t *testing.T) {
	cfg := &Config{}
	cfg.FromMap(map[string]interface{}{"cluster_name": "lab", "instance_count": 2})
	assert.Equal(t, 2, cfg.NodeCount)
}

func TestConfig_FromMap_FloatNodeCount(t *testing.T) {
	// JSON-decoded numbers arrive as float64.
	cfg := &Config{}
	cfg.FromMap(map[string]interface{}{"cluster_name": "lab", "node_count": float64(3)})
	assert.Equal(t, 3, cfg.NodeCount)
}

func TestConfig_Validate(t *testing.T) {
	valid := func() Config {
		c := Config{ClusterName: "lab"}
		c.applyDefaults()
		return c
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid", func(*Config) {}, false},
		{"missing cluster name", func(c *Config) { c.ClusterName = "" }, true},
		{"zero nodes", func(c *Config) { c.NodeCount = 0 }, true},
		{"zero cpu", func(c *Config) { c.NodeCPU = 0 }, true},
		{"zero memory", func(c *Config) { c.NodeMemoryMB = 0 }, true},
		{"zero disk", func(c *Config) { c.NodeDiskGB = 0 }, true},
		{"bad subnet", func(c *Config) { c.NetworkSubnet = "not-a-subnet" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.mutate(&c)
			err := c.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_MediaDir(t *testing.T) {
	// Default: shared version cache.
	c := &Config{}
	c.FromMap(map[string]interface{}{"cluster_name": "lab", "harvester_version": "v1.4.1"})
	assert.Equal(t, CacheDir(".", "v1.4.1"), c.MediaDir("."))

	// With an explicit ISO path: its parent directory.
	c2 := &Config{}
	c2.FromMap(map[string]interface{}{
		"cluster_name":       "lab",
		"harvester_iso_path": "/data/isos/harvester-v1.4.1-amd64.iso",
	})
	assert.Equal(t, "/data/isos", c2.MediaDir("."))
	assert.Equal(t, "/data/isos/harvester-v1.4.1-amd64.iso", c2.ISOPath)
}

func TestVagrant_GetDefaultConfig(t *testing.T) {
	d := GetDefaultConfig()
	assert.Equal(t, defaultNodeCount, d["node_count"])
	assert.Equal(t, defaultNetworkSubnet, d["network_subnet"])
}

func TestVagrant_GetRequiredFields(t *testing.T) {
	names := map[string]bool{}
	for _, f := range GetRequiredFields() {
		names[f.Name] = true
	}
	assert.True(t, names["node_count"])
	assert.True(t, names["node_prefix"])
	assert.True(t, names["network_subnet"])
}
