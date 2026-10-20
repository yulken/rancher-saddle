package harvester

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHarvesterConfig_FromMap_Defaults(t *testing.T) {
	cfg := &HarvesterConfig{}
	cfg.FromMap(map[string]interface{}{})

	assert.Equal(t, "vagrant", cfg.InstallTarget)
	assert.Equal(t, "libvirt", cfg.VagrantBackend)
	assert.GreaterOrEqual(t, cfg.NodeCount, 1)
	assert.NotEmpty(t, cfg.HarvesterVersion)
}

func TestHarvesterConfig_FromMap_Overrides(t *testing.T) {
	cfg := &HarvesterConfig{}
	cfg.FromMap(map[string]interface{}{
		"install_target":    "cloud",
		"vagrant_backend":   "libvirt",
		"harvester_version": "v1.4.0",
		"node_count":        3,
	})

	assert.Equal(t, "cloud", cfg.InstallTarget)
	assert.Equal(t, "v1.4.0", cfg.HarvesterVersion)
	assert.Equal(t, 3, cfg.NodeCount)
}

func TestHarvesterConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     HarvesterConfig
		wantErr bool
	}{
		{"valid vagrant/libvirt", HarvesterConfig{InstallTarget: "vagrant", VagrantBackend: "libvirt", NodeCount: 1}, false},
		{"valid cloud", HarvesterConfig{InstallTarget: "cloud", VagrantBackend: "libvirt", NodeCount: 3}, false},
		{"invalid install target", HarvesterConfig{InstallTarget: "azure", VagrantBackend: "libvirt", NodeCount: 1}, true},
		{"invalid backend for vagrant", HarvesterConfig{InstallTarget: "vagrant", VagrantBackend: "virtualbox", NodeCount: 1}, true},
		{"zero node count", HarvesterConfig{InstallTarget: "vagrant", VagrantBackend: "libvirt", NodeCount: 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHarvester_GetDefaultConfig(t *testing.T) {
	d := GetDefaultConfig()
	assert.Equal(t, "vagrant", d["install_target"])
	assert.Equal(t, "libvirt", d["vagrant_backend"])
	assert.Contains(t, d, "node_count")
}

func TestHarvester_GetRequiredFields(t *testing.T) {
	fields := GetRequiredFields()
	assert.NotEmpty(t, fields)
	names := map[string]bool{}
	for _, f := range fields {
		names[f.Name] = true
	}
	assert.True(t, names["install_target"])
	assert.True(t, names["vagrant_backend"])
}
