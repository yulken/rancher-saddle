package core

import (
	"context"
	"io"
)

// Provider interface for cloud providers
// Implementations handle infrastructure provisioning (VMs, networking, etc.)
type Provider interface {
	// Name returns the provider type
	Name() ProviderType

	// Validate validates the provider-specific configuration
	Validate(config map[string]interface{}) error

	// GenerateInfrastructure generates infrastructure code (e.g., Terraform) and outputs it to outputDir
	GenerateInfrastructure(ctx context.Context, config map[string]interface{}, outputDir string) error

	// GetOutputs retrieves infrastructure outputs after provisioning (IPs, DNS names, etc.)
	GetOutputs(ctx context.Context, buildDir string) (*InfrastructureOutputs, error)

	// GetRequiredFields returns the configuration fields required by this provider
	GetRequiredFields() []FormField

	// GetDefaultConfig returns default configuration values for this provider
	GetDefaultConfig() map[string]interface{}
}

// Orchestrator interface for Kubernetes distributions
// Implementations handle K8s cluster deployment and configuration
type Orchestrator interface {
	// Name returns the orchestrator type
	Name() OrchestratorType

	// Validate validates the orchestrator-specific configuration
	Validate(config map[string]interface{}) error

	// GeneratePlaybook generates the deployment playbook (e.g., Ansible) and outputs it to outputDir
	GeneratePlaybook(ctx context.Context, config map[string]interface{}, outputDir string) error

	// GenerateInventory generates the inventory file based on infrastructure outputs
	GenerateInventory(ctx context.Context, outputs *InfrastructureOutputs, config map[string]interface{}, outputDir string) error

	// GetRequiredFields returns the configuration fields required by this orchestrator
	GetRequiredFields() []FormField

	// GetDefaultConfig returns default configuration values for this orchestrator
	GetDefaultConfig() map[string]interface{}

	// GetModules returns the logical deployment modules for this orchestrator
	GetModules() []Module
}

// ProviderWithCustomApply is an optional interface a Provider can implement when
// the standard "init + apply + getOutputs + ansible" pipeline does not fit.
//
// The Vagrant/Harvester flow is the first user: it boots a PXE server VM,
// configures it via Ansible, then boots PXE nodes that self-install Harvester —
// a two-phase flow that cannot be expressed as a single `tofu apply`. When a
// provider implements this interface, the workflow runner delegates the apply
// phase to CustomApply instead of running the OpenTofu steps.
type ProviderWithCustomApply interface {
	Provider

	// PlannedOutputs returns the infrastructure outputs that are known ahead of
	// time from the configuration alone (e.g. deterministic/static IPs). The
	// runner uses these to render the inventory before the VMs are booted.
	PlannedOutputs(config map[string]interface{}) (*InfrastructureOutputs, error)

	// CustomApply performs the full provisioning flow, streaming progress to
	// logWriter, and returns the final infrastructure outputs.
	CustomApply(ctx context.Context, config map[string]interface{}, buildDir string, logWriter io.Writer) (*InfrastructureOutputs, error)
}

// SessionChecker is an optional interface a Provider can implement when AWS-style
// (or other cloud) credentials must be validated before any infrastructure work
// starts. The workflow runner calls it as a cheap preflight so an expired SSO
// session fails in seconds instead of in the middle of a `tofu apply`.
type SessionChecker interface {
	// CheckSession returns nil when the configured credentials can authenticate.
	CheckSession(ctx context.Context, config map[string]interface{}) error
}

// AuthEnvProvider is an optional interface a Provider can implement to expose
// the environment variables that tools like `tofu` must run with. Returning nil
// means "inherit the parent environment".
type AuthEnvProvider interface {
	// AuthEnv returns KEY=VALUE pairs for child processes.
	AuthEnv(config map[string]interface{}) []string
}

// Generator interface for code generation (Terraform, Ansible, etc.)
type Generator interface {
	// Generate generates code from templates
	Generate(ctx context.Context, templatePath string, data interface{}, outputPath string) error
}
