package k3s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestK3sConfig_FromMap_Defaults(t *testing.T) {
	cfg := FromMap(map[string]interface{}{})

	assert.Equal(t, defaultVersion, cfg.K3sVersion)
	assert.Equal(t, defaultRancherVersion, cfg.RancherVersion)
	assert.Equal(t, defaultBootstrapPassword, cfg.BootstrapPassword)
	assert.True(t, cfg.DeployRancher)
}

func TestK3sConfig_FromMap_VersionKey(t *testing.T) {
	cfg := FromMap(map[string]interface{}{"k3s_version": "v1.32.6+k3s1"})

	assert.Equal(t, "v1.32.6+k3s1", cfg.K3sVersion)
}

// The create form used to store the Kubernetes version under the generic
// "version" key, so every deployment fell back to the default version.
func TestK3sConfig_FromMap_LegacyVersionKey(t *testing.T) {
	cfg := FromMap(map[string]interface{}{"version": "v1.32.6+k3s1"})

	assert.Equal(t, "v1.32.6+k3s1", cfg.K3sVersion)
}

func TestK3sConfig_FromMap_CanonicalKeyWins(t *testing.T) {
	cfg := FromMap(map[string]interface{}{
		"k3s_version": "v1.32.6+k3s1",
		"version":     "v1.35.0+k3s1",
	})

	assert.Equal(t, "v1.32.6+k3s1", cfg.K3sVersion)
}

func TestK3sConfig_FromMap_EmptyVersionFallsBackToDefault(t *testing.T) {
	cfg := FromMap(map[string]interface{}{"k3s_version": ""})

	assert.Equal(t, defaultVersion, cfg.K3sVersion)
}

func TestGetRequiredFields_VersionDefaults(t *testing.T) {
	defaults := map[string]interface{}{}
	for _, field := range GetRequiredFields() {
		defaults[field.Name] = field.Default
	}

	assert.Equal(t, defaultVersion, defaults["k3s_version"])
	assert.Equal(t, defaultRancherVersion, defaults["rancher_version"])
}
