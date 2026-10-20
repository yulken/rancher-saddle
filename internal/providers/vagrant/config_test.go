package vagrant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVagrantConfig_FromMap_Defaults(t *testing.T) {
	cfg := &VagrantConfig{}
	cfg.FromMap(map[string]interface{}{})

	assert.Equal(t, "libvirt", cfg.Backend)
	assert.Greater(t, cfg.Memory, 0)
	assert.Greater(t, cfg.CPUs, 0)
	assert.NotEmpty(t, cfg.Box)
}

func TestVagrantConfig_FromMap_Overrides(t *testing.T) {
	cfg := &VagrantConfig{}
	cfg.FromMap(map[string]interface{}{
		"backend": "libvirt",
		"memory":  8192,
		"cpus":    4,
		"box":     "opensuse/Leap-15.6.x86_64",
	})

	assert.Equal(t, 8192, cfg.Memory)
	assert.Equal(t, 4, cfg.CPUs)
	assert.Equal(t, "opensuse/Leap-15.6.x86_64", cfg.Box)
}

func TestVagrantConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     VagrantConfig
		wantErr bool
	}{
		{"valid libvirt", VagrantConfig{Backend: "libvirt", Memory: 4096, CPUs: 2}, false},
		{"unsupported backend", VagrantConfig{Backend: "virtualbox", Memory: 4096, CPUs: 2}, true},
		{"zero memory", VagrantConfig{Backend: "libvirt", Memory: 0, CPUs: 2}, true},
		{"zero cpus", VagrantConfig{Backend: "libvirt", Memory: 4096, CPUs: 0}, true},
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

func TestVagrant_GetDefaultConfig(t *testing.T) {
	d := GetDefaultConfig()
	assert.Equal(t, "libvirt", d["backend"])
	assert.Contains(t, d, "memory")
	assert.Contains(t, d, "cpus")
}
