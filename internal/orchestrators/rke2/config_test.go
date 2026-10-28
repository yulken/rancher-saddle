package rke2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRKE2Config_FromMap_Defaults(t *testing.T) {
	cfg := &RKE2Config{}
	cfg.FromMap(map[string]interface{}{})

	assert.Equal(t, defaultVersion, cfg.RKE2Version)
	assert.Equal(t, defaultRancherVersion, cfg.RancherVersion)
	assert.Equal(t, defaultBootstrapPassword, cfg.BootstrapPassword)
	assert.True(t, cfg.DeployRancher)
}

func TestRKE2Config_FromMap_VersionKey(t *testing.T) {
	cfg := &RKE2Config{}
	cfg.FromMap(map[string]interface{}{"rke2_version": "v1.32.6+rke2r1"})

	assert.Equal(t, "v1.32.6+rke2r1", cfg.RKE2Version)
}

// The create form used to store the Kubernetes version under the generic
// "version" key, so every deployment fell back to the default version.
func TestRKE2Config_FromMap_LegacyVersionKey(t *testing.T) {
	cfg := &RKE2Config{}
	cfg.FromMap(map[string]interface{}{"version": "v1.32.6+rke2r1"})

	assert.Equal(t, "v1.32.6+rke2r1", cfg.RKE2Version)
}

func TestRKE2Config_FromMap_CanonicalKeyWins(t *testing.T) {
	cfg := &RKE2Config{}
	cfg.FromMap(map[string]interface{}{
		"rke2_version": "v1.32.6+rke2r1",
		"version":      "v1.35.0+rke2r1",
	})

	assert.Equal(t, "v1.32.6+rke2r1", cfg.RKE2Version)
}

func TestRKE2Config_FromMap_EmptyVersionFallsBackToDefault(t *testing.T) {
	cfg := &RKE2Config{}
	cfg.FromMap(map[string]interface{}{"rke2_version": ""})

	assert.Equal(t, defaultVersion, cfg.RKE2Version)
}

func TestGetRequiredFields_VersionDefaults(t *testing.T) {
	defaults := map[string]interface{}{}
	for _, field := range GetRequiredFields() {
		defaults[field.Name] = field.Default
	}

	assert.Equal(t, defaultVersion, defaults["rke2_version"])
	assert.Equal(t, defaultRancherVersion, defaults["rancher_version"])
}

func TestGetDefaultConfig(t *testing.T) {
	got := GetDefaultConfig()

	assert.Equal(t, defaultVersion, got["rke2_version"])
	assert.Equal(t, defaultRancherVersion, got["rancher_version"])
}
