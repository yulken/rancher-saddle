package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeployConfig_ContainerName(t *testing.T) {
	cfg := DeployConfig{ClusterName: "my-cluster"}
	assert.Equal(t, "rancher-my-cluster", cfg.ContainerName())
}

func TestDeployConfig_VolumeName(t *testing.T) {
	cfg := DeployConfig{ClusterName: "my-cluster"}
	assert.Equal(t, "rancher-data-my-cluster", cfg.VolumeName())
}

func TestDeployConfig_Image(t *testing.T) {
	tests := []struct {
		name     string
		cfg      DeployConfig
		expected string
	}{
		{
			name:     "community with version",
			cfg:      DeployConfig{RancherVersion: "2.11.7"},
			expected: "rancher/rancher:v2.11.7",
		},
		{
			name:     "prime with version",
			cfg:      DeployConfig{RancherVersion: "2.11.7", Prime: true},
			expected: "registry.suse.com/rancher/rancher:v2.11.7",
		},
		{
			name:     "image tag overrides version",
			cfg:      DeployConfig{RancherVersion: "2.11.7", ImageTag: "v0.0.0-hotfix-abc123.1"},
			expected: "rancher/rancher:v0.0.0-hotfix-abc123.1",
		},
		{
			name:     "prime with image tag override",
			cfg:      DeployConfig{RancherVersion: "2.11.7", Prime: true, ImageTag: "v0.0.0-hotfix-abc123.1"},
			expected: "registry.suse.com/rancher/rancher:v0.0.0-hotfix-abc123.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cfg.Image())
		})
	}
}

func TestBuildRunArgs(t *testing.T) {
	tests := []struct {
		name         string
		cfg          DeployConfig
		wantContains []string
		wantAbsent   []string
	}{
		{
			name: "community defaults",
			cfg: DeployConfig{
				ClusterName:       "test",
				RancherVersion:    "2.11.7",
				BootstrapPassword: "admin",
				HostPort:          "443",
			},
			wantContains: []string{
				"run", "-d",
				"--name", "rancher-test",
				"--privileged",
				"--restart=unless-stopped",
				"-p", "443:443",
				"-v", "rancher-data-test:/var/lib/rancher",
				"-e", "CATTLE_BOOTSTRAP_PASSWORD=admin",
				"rancher/rancher:v2.11.7",
			},
			wantAbsent: []string{
				"RANCHER_VERSION_TYPE=prime",
				"CATTLE_BASE_UI_BRAND=suse",
				"CATTLE_DEBUG=true",
			},
		},
		{
			name: "prime with debug",
			cfg: DeployConfig{
				ClusterName:       "prod",
				RancherVersion:    "2.10.2",
				BootstrapPassword: "secret",
				Prime:             true,
				Debug:             true,
				HostPort:          "443",
			},
			wantContains: []string{
				"--name", "rancher-prod",
				"CATTLE_BOOTSTRAP_PASSWORD=secret",
				"CATTLE_DEBUG=true",
				"RANCHER_VERSION_TYPE=prime",
				"CATTLE_BASE_UI_BRAND=suse",
				"registry.suse.com/rancher/rancher:v2.10.2",
			},
		},
		{
			name: "custom port",
			cfg: DeployConfig{
				ClusterName:       "dev",
				RancherVersion:    "2.11.7",
				BootstrapPassword: "admin",
				HostPort:          "8443",
			},
			wantContains: []string{
				"-p", "8443:443",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := BuildRunArgs(tt.cfg)
			argsStr := joinArgs(args)

			for _, want := range tt.wantContains {
				assert.Contains(t, argsStr, want, "args should contain %q", want)
			}

			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, argsStr, absent, "args should NOT contain %q", absent)
			}
		})
	}
}

func TestDefaultDeployConfig(t *testing.T) {
	cfg := DefaultDeployConfig()
	assert.Equal(t, "2.11.7", cfg.RancherVersion)
	assert.Equal(t, "admin", cfg.BootstrapPassword)
	assert.Equal(t, "443", cfg.HostPort)
	assert.False(t, cfg.Prime)
	assert.False(t, cfg.Debug)
}

// joinArgs joins args into a single string for substring matching.
func joinArgs(args []string) string {
	result := ""
	for _, a := range args {
		result += a + " "
	}
	return result
}
