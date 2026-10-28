package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ClustersConfig represents the entire config.yaml file with all clusters
type ClustersConfig struct {
	Clusters map[string]*ClusterConfig `yaml:"clusters"`
}

// ClusterConfig represents a single cluster configuration
type ClusterConfig struct {
	Provider    ProviderSection   `yaml:"provider"`
	Kubernetes  KubernetesSection `yaml:"kubernetes"`
	Rancher     RancherSection    `yaml:"rancher"`
	SSH         SSHSection        `yaml:"ssh"`
	Cluster     ClusterSection    `yaml:"cluster"`
	Status      string            `yaml:"status,omitempty"`
	CreatedAt   time.Time         `yaml:"created_at,omitempty"`
	UpdatedAt   time.Time         `yaml:"updated_at,omitempty"`
	BuildDir    string            `yaml:"build_dir,omitempty"`
	InstanceIPs []string          `yaml:"instance_ips,omitempty"`
	InstanceDNS []string          `yaml:"instance_dns,omitempty"`
	RancherURL  string            `yaml:"rancher_url,omitempty"`
	LastError   string            `yaml:"last_error,omitempty"`
}

// ProviderSection contains cloud provider configuration
type ProviderSection struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

// KubernetesSection contains Kubernetes distribution configuration
type KubernetesSection struct {
	Distribution string                 `yaml:"distribution"`
	Config       map[string]interface{} `yaml:"config"`
}

// RancherSection contains Rancher configuration
type RancherSection struct {
	Version           string `yaml:"version"`
	Deploy            bool   `yaml:"deploy"`
	Prime             bool   `yaml:"prime"`
	BootstrapPassword string `yaml:"bootstrap_password"`
	AuditLog          bool   `yaml:"audit_log,omitempty"`
	AuditLogLevel     int    `yaml:"audit_log_level,omitempty"`
	ImageTag          string `yaml:"image_tag,omitempty"`
	Debug             bool   `yaml:"debug,omitempty"`
}

// SSHSection contains SSH configuration
type SSHSection struct {
	KeyName        string `yaml:"key_name"`
	PrivateKeyPath string `yaml:"private_key_path"`
	User           string `yaml:"user"`
}

// ClusterSection contains general cluster configuration
type ClusterSection struct {
	NodePrefix    string `yaml:"node_prefix"`
	InstanceCount int    `yaml:"instance_count"`
}

// LoadClustersConfig loads the config.yaml file from project root
func LoadClustersConfig(path string) (*ClustersConfig, error) {
	// If file doesn't exist, return empty config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &ClustersConfig{
			Clusters: make(map[string]*ClusterConfig),
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ClustersConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Clusters == nil {
		cfg.Clusters = make(map[string]*ClusterConfig)
	}

	return &cfg, nil
}

// Save writes the config to the specified path
func (c *ClustersConfig) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// File permission 0600 because it contains secrets
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// AddCluster adds or updates a cluster in the config
func (c *ClustersConfig) AddCluster(name string, cluster *ClusterConfig) {
	if c.Clusters == nil {
		c.Clusters = make(map[string]*ClusterConfig)
	}
	cluster.UpdatedAt = time.Now()
	if cluster.CreatedAt.IsZero() {
		cluster.CreatedAt = time.Now()
	}
	c.Clusters[name] = cluster
}

// GetCluster retrieves a cluster by name
func (c *ClustersConfig) GetCluster(name string) (*ClusterConfig, bool) {
	cluster, exists := c.Clusters[name]
	return cluster, exists
}

// DeleteCluster removes a cluster from the config
func (c *ClustersConfig) DeleteCluster(name string) {
	delete(c.Clusters, name)
}

// ListClusters returns all cluster names sorted alphabetically
func (c *ClustersConfig) ListClusters() []string {
	names := make([]string, 0, len(c.Clusters))
	for name := range c.Clusters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// VersionKeyFor returns the orchestrator config key that holds the Kubernetes
// version for the given distribution. The orchestrators only read their own
// key, so anything written under a different one is silently ignored.
func VersionKeyFor(distribution string) string {
	switch strings.ToLower(distribution) {
	case "rke2":
		return "rke2_version"
	case "k3s":
		return "k3s_version"
	case "harvester":
		return "harvester_version"
	default:
		return "version"
	}
}

// rancherConfigKeys live in the orchestrator config map while a deployment runs
// but are persisted under the Rancher section of a ClusterConfig.
var rancherConfigKeys = []string{
	"rancher_version",
	"deploy_rancher",
	"rancher_prime",
	"rancher_bootstrap_password",
	"rancher_image_tag",
	"rancher_debug",
}

// copyConfigMap returns a shallow copy so callers can add or remove keys
// without mutating the cluster config (and, through it, config.yaml).
func copyConfigMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in)+len(rancherConfigKeys))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// normalizeVersionKey copies the legacy "version" key into the key the
// orchestrator actually reads. Earlier builds of the create form stored the
// Kubernetes version under "version", which made every deployment fall back to
// the hardcoded default.
func normalizeVersionKey(config map[string]interface{}, distribution string) {
	key := VersionKeyFor(distribution)
	if key == "version" {
		return
	}
	if _, ok := config[key]; ok {
		return
	}
	if v, ok := config["version"].(string); ok && v != "" {
		config[key] = v
	}
}

// orchestratorSpecificConfig drops the Rancher keys from an orchestrator config
// map so they are not duplicated under kubernetes.config when the cluster is
// written back to config.yaml.
func orchestratorSpecificConfig(in map[string]interface{}) map[string]interface{} {
	out := copyConfigMap(in)
	for _, key := range rancherConfigKeys {
		delete(out, key)
	}
	return out
}

// ToModernConfig converts a ClusterConfig to the modern Config format for workflows
func (cc *ClusterConfig) ToModernConfig() *Config {
	orchestratorConfig := copyConfigMap(cc.Kubernetes.Config)
	normalizeVersionKey(orchestratorConfig, cc.Kubernetes.Distribution)

	cfg := &Config{
		Provider:           cc.Provider.Type,
		Orchestrator:       cc.Kubernetes.Distribution,
		NodePrefix:         cc.Cluster.NodePrefix,
		InstanceCount:      cc.Cluster.InstanceCount,
		SSHKeyName:         cc.SSH.KeyName,
		SSHPrivateKeyPath:  cc.SSH.PrivateKeyPath,
		SSHUser:            cc.SSH.User,
		ProviderConfig:     cc.Provider.Config,
		OrchestratorConfig: orchestratorConfig,
		Addons:             []string{},
	}

	cfg.OrchestratorConfig["rancher_version"] = cc.Rancher.Version
	cfg.OrchestratorConfig["deploy_rancher"] = cc.Rancher.Deploy
	cfg.OrchestratorConfig["rancher_prime"] = cc.Rancher.Prime
	cfg.OrchestratorConfig["rancher_bootstrap_password"] = cc.Rancher.BootstrapPassword
	cfg.OrchestratorConfig["rancher_image_tag"] = cc.Rancher.ImageTag
	cfg.OrchestratorConfig["rancher_debug"] = cc.Rancher.Debug

	return cfg
}

// FromModernConfig creates a ClusterConfig from the modern Config format
func FromModernConfig(cfg *Config) *ClusterConfig {
	cc := &ClusterConfig{
		Provider: ProviderSection{
			Type:   cfg.Provider,
			Config: cfg.ProviderConfig,
		},
		Kubernetes: KubernetesSection{
			Distribution: cfg.Orchestrator,
			Config:       orchestratorSpecificConfig(cfg.OrchestratorConfig),
		},
		SSH: SSHSection{
			KeyName:        cfg.SSHKeyName,
			PrivateKeyPath: cfg.SSHPrivateKeyPath,
			User:           cfg.SSHUser,
		},
		Cluster: ClusterSection{
			NodePrefix:    cfg.NodePrefix,
			InstanceCount: cfg.InstanceCount,
		},
	}

	// Extract Rancher config from orchestrator config
	if cfg.OrchestratorConfig != nil {
		if v, ok := cfg.OrchestratorConfig["rancher_version"].(string); ok {
			cc.Rancher.Version = v
		}
		if v, ok := cfg.OrchestratorConfig["deploy_rancher"].(bool); ok {
			cc.Rancher.Deploy = v
		}
		if v, ok := cfg.OrchestratorConfig["rancher_prime"].(bool); ok {
			cc.Rancher.Prime = v
		}
		if v, ok := cfg.OrchestratorConfig["rancher_bootstrap_password"].(string); ok {
			cc.Rancher.BootstrapPassword = v
		}
		if v, ok := cfg.OrchestratorConfig["rancher_image_tag"].(string); ok {
			cc.Rancher.ImageTag = v
		}
		if v, ok := cfg.OrchestratorConfig["rancher_debug"].(bool); ok {
			cc.Rancher.Debug = v
		}
	}

	return cc
}
