package core

import (
	"context"
)

// ProviderType represents the cloud provider type
type ProviderType string

const (
	ProviderAWS     ProviderType = "aws"
	ProviderAzure   ProviderType = "azure"
	ProviderGCP     ProviderType = "gcp"
	ProviderVSphere ProviderType = "vsphere"
	ProviderLibvirt ProviderType = "libvirt"
)

// OrchestratorType represents the Kubernetes distribution type
type OrchestratorType string

const (
	OrchestratorRKE2      OrchestratorType = "rke2"
	OrchestratorK3s       OrchestratorType = "k3s"
	OrchestratorDocker    OrchestratorType = "docker"
	OrchestratorMinikube  OrchestratorType = "minikube"
	OrchestratorKubeadm   OrchestratorType = "kubeadm"
	OrchestratorHarvester OrchestratorType = "harvester"
)

// FormField represents a configuration field for TUI forms
type FormField struct {
	Name        string
	Label       string
	Description string
	Required    bool
	Default     interface{}
	Type        string   // "string", "int", "bool", "select"
	Options     []string // For select type
}

// InfrastructureOutputs holds the outputs from infrastructure provisioning
type InfrastructureOutputs struct {
	InstanceIPs       []string
	InstanceDNSNames  []string
	PrivateIPs        []string
	AdditionalOutputs map[string]interface{}
}

// Module represents a logical deployment module for orchestrators
type Module interface {
	Name() string
	Description() string
	Generate(ctx context.Context, config map[string]interface{}) (string, error)
}
