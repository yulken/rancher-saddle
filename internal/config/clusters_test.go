package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClustersConfig_AddAndGet(t *testing.T) {
	cfg := &ClustersConfig{Clusters: make(map[string]*ClusterConfig)}

	cluster := &ClusterConfig{
		Provider:   ProviderSection{Type: "aws"},
		Kubernetes: KubernetesSection{Distribution: "rke2"},
		Status:     "running",
	}

	cfg.AddCluster("test-cluster", cluster)

	got, exists := cfg.GetCluster("test-cluster")
	assert.True(t, exists)
	assert.Equal(t, "aws", got.Provider.Type)
	assert.Equal(t, "rke2", got.Kubernetes.Distribution)
	assert.False(t, got.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, got.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestClustersConfig_GetNotFound(t *testing.T) {
	cfg := &ClustersConfig{Clusters: make(map[string]*ClusterConfig)}

	_, exists := cfg.GetCluster("nonexistent")
	assert.False(t, exists)
}

func TestClustersConfig_Delete(t *testing.T) {
	cfg := &ClustersConfig{Clusters: make(map[string]*ClusterConfig)}
	cfg.AddCluster("to-delete", &ClusterConfig{})

	cfg.DeleteCluster("to-delete")

	_, exists := cfg.GetCluster("to-delete")
	assert.False(t, exists)
}

func TestClustersConfig_ListSorted(t *testing.T) {
	cfg := &ClustersConfig{Clusters: make(map[string]*ClusterConfig)}
	cfg.AddCluster("charlie", &ClusterConfig{})
	cfg.AddCluster("alpha", &ClusterConfig{})
	cfg.AddCluster("bravo", &ClusterConfig{})

	names := cfg.ListClusters()
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, names)
}

func TestClustersConfig_AddNilMap(t *testing.T) {
	cfg := &ClustersConfig{}
	cfg.AddCluster("test", &ClusterConfig{})

	got, exists := cfg.GetCluster("test")
	assert.True(t, exists)
	assert.NotNil(t, got)
}

func TestClusterConfig_ToModernConfig(t *testing.T) {
	cc := &ClusterConfig{
		Provider: ProviderSection{
			Type:   "aws",
			Config: map[string]interface{}{"region": "us-east-1"},
		},
		Kubernetes: KubernetesSection{
			Distribution: "rke2",
			Config:       map[string]interface{}{"version": "v1.33.7"},
		},
		Rancher: RancherSection{
			Version:           "2.11.7",
			Deploy:            true,
			Prime:             true,
			BootstrapPassword: "secret",
		},
		SSH: SSHSection{
			KeyName:        "my-key",
			PrivateKeyPath: "/tmp/key.pem",
			User:           "ubuntu",
		},
		Cluster: ClusterSection{
			NodePrefix:    "k8s-node",
			InstanceCount: 3,
		},
	}

	cfg := cc.ToModernConfig()

	assert.Equal(t, "aws", cfg.Provider)
	assert.Equal(t, "rke2", cfg.Orchestrator)
	assert.Equal(t, "k8s-node", cfg.NodePrefix)
	assert.Equal(t, 3, cfg.InstanceCount)
	assert.Equal(t, "my-key", cfg.SSHKeyName)
	assert.Equal(t, "/tmp/key.pem", cfg.SSHPrivateKeyPath)
	assert.Equal(t, "ubuntu", cfg.SSHUser)
	assert.Equal(t, "us-east-1", cfg.ProviderConfig["region"])
	assert.Equal(t, "2.11.7", cfg.OrchestratorConfig["rancher_version"])
	assert.Equal(t, true, cfg.OrchestratorConfig["deploy_rancher"])
	assert.Equal(t, true, cfg.OrchestratorConfig["rancher_prime"])
	assert.Equal(t, "secret", cfg.OrchestratorConfig["rancher_bootstrap_password"])
}

func TestClusterConfig_ToModernConfig_NilOrchestratorConfig(t *testing.T) {
	cc := &ClusterConfig{
		Kubernetes: KubernetesSection{Distribution: "k3s"},
		Rancher:    RancherSection{Version: "2.11.7", Deploy: true},
	}

	cfg := cc.ToModernConfig()

	assert.NotNil(t, cfg.OrchestratorConfig)
	assert.Equal(t, "2.11.7", cfg.OrchestratorConfig["rancher_version"])
}

func TestFromModernConfig(t *testing.T) {
	cfg := &Config{
		Provider:          "aws",
		Orchestrator:      "rke2",
		NodePrefix:        "node",
		InstanceCount:     3,
		SSHKeyName:        "key",
		SSHPrivateKeyPath: "/path/key.pem",
		SSHUser:           "ec2-user",
		ProviderConfig:    map[string]interface{}{"region": "eu-west-1"},
		OrchestratorConfig: map[string]interface{}{
			"rancher_version":            "2.11.7",
			"deploy_rancher":             true,
			"rancher_prime":              false,
			"rancher_bootstrap_password": "admin",
		},
	}

	cc := FromModernConfig(cfg)

	assert.Equal(t, "aws", cc.Provider.Type)
	assert.Equal(t, "rke2", cc.Kubernetes.Distribution)
	assert.Equal(t, "node", cc.Cluster.NodePrefix)
	assert.Equal(t, 3, cc.Cluster.InstanceCount)
	assert.Equal(t, "key", cc.SSH.KeyName)
	assert.Equal(t, "2.11.7", cc.Rancher.Version)
	assert.Equal(t, true, cc.Rancher.Deploy)
	assert.Equal(t, false, cc.Rancher.Prime)
	assert.Equal(t, "admin", cc.Rancher.BootstrapPassword)
}

func TestFromModernConfig_NilOrchestratorConfig(t *testing.T) {
	cfg := &Config{
		Provider:     "aws",
		Orchestrator: "rke2",
	}

	cc := FromModernConfig(cfg)

	assert.Equal(t, "", cc.Rancher.Version)
	assert.Equal(t, false, cc.Rancher.Deploy)
}

func TestClustersConfig_LoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := &ClustersConfig{Clusters: make(map[string]*ClusterConfig)}
	original.AddCluster("my-cluster", &ClusterConfig{
		Provider:   ProviderSection{Type: "aws", Config: map[string]interface{}{"region": "us-east-1"}},
		Kubernetes: KubernetesSection{Distribution: "rke2"},
		Rancher:    RancherSection{Version: "2.11.7", Deploy: true, Prime: false, BootstrapPassword: "admin"},
		SSH:        SSHSection{KeyName: "key", PrivateKeyPath: "/tmp/key.pem", User: "ubuntu"},
		Cluster:    ClusterSection{NodePrefix: "node", InstanceCount: 3},
		Status:     "running",
	})

	err := original.Save(path)
	require.NoError(t, err)

	loaded, err := LoadClustersConfig(path)
	require.NoError(t, err)

	got, exists := loaded.GetCluster("my-cluster")
	assert.True(t, exists)
	assert.Equal(t, "aws", got.Provider.Type)
	assert.Equal(t, "rke2", got.Kubernetes.Distribution)
	assert.Equal(t, "2.11.7", got.Rancher.Version)
	assert.Equal(t, true, got.Rancher.Deploy)
	assert.Equal(t, 3, got.Cluster.InstanceCount)
	assert.Equal(t, "running", got.Status)
}

func TestClusterConfig_ToModernConfig_ImageTagAndDebug(t *testing.T) {
	cc := &ClusterConfig{
		Kubernetes: KubernetesSection{Distribution: "rke2"},
		Rancher: RancherSection{
			Version:  "2.13.3",
			Deploy:   true,
			Prime:    true,
			ImageTag: "v0.0.0-test-abc123.1",
			Debug:    true,
		},
	}

	cfg := cc.ToModernConfig()

	assert.Equal(t, "v0.0.0-test-abc123.1", cfg.OrchestratorConfig["rancher_image_tag"])
	assert.Equal(t, true, cfg.OrchestratorConfig["rancher_debug"])
}

func TestFromModernConfig_ImageTagAndDebug(t *testing.T) {
	cfg := &Config{
		Provider:     "aws",
		Orchestrator: "rke2",
		OrchestratorConfig: map[string]interface{}{
			"rancher_version":   "2.13.3",
			"deploy_rancher":    true,
			"rancher_image_tag": "v0.0.0-test-abc123.1",
			"rancher_debug":     true,
		},
	}

	cc := FromModernConfig(cfg)

	assert.Equal(t, "v0.0.0-test-abc123.1", cc.Rancher.ImageTag)
	assert.Equal(t, true, cc.Rancher.Debug)
}

func TestClustersConfig_SaveLoad_ImageTagAndDebug(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := &ClustersConfig{Clusters: make(map[string]*ClusterConfig)}
	original.AddCluster("hotfix-cluster", &ClusterConfig{
		Provider:   ProviderSection{Type: "aws"},
		Kubernetes: KubernetesSection{Distribution: "rke2"},
		Rancher: RancherSection{
			Version:  "2.13.3",
			Deploy:   true,
			Prime:    true,
			ImageTag: "v0.0.0-test-abc123.1",
			Debug:    true,
		},
		Status: "running",
	})

	err := original.Save(path)
	require.NoError(t, err)

	loaded, err := LoadClustersConfig(path)
	require.NoError(t, err)

	got, exists := loaded.GetCluster("hotfix-cluster")
	assert.True(t, exists)
	assert.Equal(t, "v0.0.0-test-abc123.1", got.Rancher.ImageTag)
	assert.Equal(t, true, got.Rancher.Debug)
}

func TestClustersConfig_SaveLoad_EmptyImageTag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := &ClustersConfig{Clusters: make(map[string]*ClusterConfig)}
	original.AddCluster("normal-cluster", &ClusterConfig{
		Provider:   ProviderSection{Type: "aws"},
		Kubernetes: KubernetesSection{Distribution: "rke2"},
		Rancher: RancherSection{
			Version: "2.11.7",
			Deploy:  true,
		},
		Status: "running",
	})

	err := original.Save(path)
	require.NoError(t, err)

	loaded, err := LoadClustersConfig(path)
	require.NoError(t, err)

	got, exists := loaded.GetCluster("normal-cluster")
	assert.True(t, exists)
	assert.Equal(t, "", got.Rancher.ImageTag)
	assert.Equal(t, false, got.Rancher.Debug)
}

func TestLoadClustersConfig_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.yaml")

	cfg, err := LoadClustersConfig(path)
	require.NoError(t, err)
	assert.NotNil(t, cfg.Clusters)
	assert.Empty(t, cfg.Clusters)
}

func TestVersionKeyFor(t *testing.T) {
	cases := map[string]string{
		"rke2":      "rke2_version",
		"RKE2":      "rke2_version",
		"k3s":       "k3s_version",
		"K3s":       "k3s_version",
		"harvester": "harvester_version",
		"docker":    "version",
		"":          "version",
	}

	for distribution, want := range cases {
		assert.Equal(t, want, VersionKeyFor(distribution), "distribution %q", distribution)
	}
}

// The create form used to store the Kubernetes version under "version", which
// the orchestrators never read: deployments silently used their default.
func TestClusterConfig_ToModernConfig_NormalizesLegacyVersionKey(t *testing.T) {
	cc := &ClusterConfig{
		Kubernetes: KubernetesSection{
			Distribution: "rke2",
			Config:       map[string]interface{}{"version": "v1.32.6+rke2r1"},
		},
	}

	cfg := cc.ToModernConfig()

	assert.Equal(t, "v1.32.6+rke2r1", cfg.OrchestratorConfig["rke2_version"])
}

func TestClusterConfig_ToModernConfig_KeepsCanonicalVersionKey(t *testing.T) {
	cc := &ClusterConfig{
		Kubernetes: KubernetesSection{
			Distribution: "k3s",
			Config: map[string]interface{}{
				"k3s_version": "v1.32.6+k3s1",
				"version":     "v1.35.0+k3s1",
			},
		},
	}

	cfg := cc.ToModernConfig()

	assert.Equal(t, "v1.32.6+k3s1", cfg.OrchestratorConfig["k3s_version"])
}

func TestClusterConfig_ToModernConfig_DoesNotMutateKubernetesConfig(t *testing.T) {
	original := map[string]interface{}{"version": "v1.32.6+rke2r1"}
	cc := &ClusterConfig{
		Kubernetes: KubernetesSection{Distribution: "rke2", Config: original},
		Rancher:    RancherSection{Version: "2.11.14", Deploy: true},
	}

	cc.ToModernConfig()

	assert.Equal(t, map[string]interface{}{"version": "v1.32.6+rke2r1"}, cc.Kubernetes.Config)
}

func TestFromModernConfig_KeepsRancherKeysOutOfKubernetesConfig(t *testing.T) {
	cfg := &Config{
		Provider:     "aws",
		Orchestrator: "rke2",
		OrchestratorConfig: map[string]interface{}{
			"rke2_version":    "v1.32.6+rke2r1",
			"rancher_version": "2.11.14",
			"deploy_rancher":  true,
			"rancher_prime":   true,
		},
	}

	cc := FromModernConfig(cfg)

	assert.Equal(t, "v1.32.6+rke2r1", cc.Kubernetes.Config["rke2_version"])
	for _, key := range rancherConfigKeys {
		assert.NotContains(t, cc.Kubernetes.Config, key)
	}
	assert.Equal(t, "2.11.14", cc.Rancher.Version)
	assert.Equal(t, true, cc.Rancher.Deploy)
	assert.Equal(t, true, cc.Rancher.Prime)
}

func TestClusterConfig_ModernConfigRoundTrip(t *testing.T) {
	cc := &ClusterConfig{
		Provider:   ProviderSection{Type: "aws"},
		Kubernetes: KubernetesSection{Distribution: "rke2", Config: map[string]interface{}{"rke2_version": "v1.32.6+rke2r1"}},
		Rancher:    RancherSection{Version: "2.11.14", Deploy: true},
	}

	back := FromModernConfig(cc.ToModernConfig())

	assert.Equal(t, "rke2", back.Kubernetes.Distribution)
	assert.Equal(t, "v1.32.6+rke2r1", back.Kubernetes.Config["rke2_version"])
	assert.Equal(t, "2.11.14", back.Rancher.Version)
	assert.Equal(t, true, back.Rancher.Deploy)
}
