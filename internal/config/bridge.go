package config

import (
	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/model"
)

// FromLegacyConfig converts old model.Config to new config.Config
func FromLegacyConfig(oldCfg *model.Config) *Config {
	cfg := &Config{
		Provider:           string(core.ProviderAWS),
		Orchestrator:       oldCfg.KubernetesDistribution, // Dynamic based on selection
		NodePrefix:         oldCfg.NodePrefix,
		InstanceCount:      oldCfg.InstanceCount,
		SSHKeyName:         oldCfg.SSHKeyName,
		SSHPrivateKeyPath:  oldCfg.SSHPrivateKeyPath,
		SSHUser:            "ubuntu",
		ProviderConfig:     make(map[string]interface{}),
		OrchestratorConfig: make(map[string]interface{}),
		Addons:             []string{},
	}

	// Map AWS-specific fields
	cfg.ProviderConfig["access_key"] = oldCfg.AWSAccessKey
	cfg.ProviderConfig["secret_key"] = oldCfg.AWSSecretKey
	cfg.ProviderConfig["region"] = oldCfg.AWSRegion
	cfg.ProviderConfig["subnet_id"] = oldCfg.SubnetID
	cfg.ProviderConfig["security_group_id"] = oldCfg.SecurityGroupID
	cfg.ProviderConfig["ami"] = oldCfg.AMI
	cfg.ProviderConfig["instance_type"] = "t3.xlarge"
	cfg.ProviderConfig["root_volume_size"] = oldCfg.RootVolumeSize

	// Map orchestrator-specific fields based on distribution
	if oldCfg.KubernetesDistribution == "rke2" {
		cfg.OrchestratorConfig["rke2_version"] = oldCfg.KubernetesVersion
		cfg.OrchestratorConfig["rancher_version"] = oldCfg.RancherVersion
		cfg.OrchestratorConfig["deploy_rancher"] = true
	} else if oldCfg.KubernetesDistribution == "k3s" {
		cfg.OrchestratorConfig["k3s_version"] = oldCfg.KubernetesVersion
		cfg.OrchestratorConfig["rancher_version"] = oldCfg.RancherVersion
		cfg.OrchestratorConfig["deploy_rancher"] = true
	}

	return cfg
}

// ToLegacyConfig converts new config.Config to old model.Config
func ToLegacyConfig(newCfg *Config) *model.Config {
	cfg := &model.Config{
		NodePrefix:        newCfg.NodePrefix,
		InstanceCount:     newCfg.InstanceCount,
		SSHKeyName:        newCfg.SSHKeyName,
		SSHPrivateKeyPath: newCfg.SSHPrivateKeyPath,
	}

	// Extract AWS-specific fields
	if v, ok := newCfg.ProviderConfig["access_key"].(string); ok {
		cfg.AWSAccessKey = v
	}
	if v, ok := newCfg.ProviderConfig["secret_key"].(string); ok {
		cfg.AWSSecretKey = v
	}
	if v, ok := newCfg.ProviderConfig["region"].(string); ok {
		cfg.AWSRegion = v
	}
	if v, ok := newCfg.ProviderConfig["subnet_id"].(string); ok {
		cfg.SubnetID = v
	}
	if v, ok := newCfg.ProviderConfig["security_group_id"].(string); ok {
		cfg.SecurityGroupID = v
	}
	if v, ok := newCfg.ProviderConfig["ami"].(string); ok {
		cfg.AMI = v
	}
	if v, ok := newCfg.ProviderConfig["root_volume_size"].(float64); ok {
		cfg.RootVolumeSize = int(v)
	} else if v, ok := newCfg.ProviderConfig["root_volume_size"].(int); ok {
		cfg.RootVolumeSize = v
	}

	// Extract RKE2-specific fields
	if v, ok := newCfg.OrchestratorConfig["rke2_version"].(string); ok {
		cfg.RKE2Version = v
	}
	if v, ok := newCfg.OrchestratorConfig["rancher_version"].(string); ok {
		cfg.RancherVersion = v
	}

	return cfg
}
