// Package vagrant scaffolds a Vagrant-based local VM provider.
//
// Phase 1 provides config, validation, form fields and registry wiring so
// Vagrant is selectable (e.g. for local Harvester installs on libvirt/KVM), but
// infrastructure generation and output retrieval are not implemented yet
// (return ErrNotImplemented). See feats/2-harvester-vagrant.md.
package vagrant

import (
	"context"
	"errors"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// ErrNotImplemented is returned by generation methods that are not yet built.
var ErrNotImplemented = errors.New("vagrant: not implemented in Phase 1")

// Provider implements the Vagrant local provider (scaffold).
type Provider struct{}

// NewProvider creates a new Vagrant provider instance.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the provider type.
func (p *Provider) Name() core.ProviderType {
	return core.ProviderVagrant
}

// Validate validates the Vagrant configuration.
func (p *Provider) Validate(config map[string]interface{}) error {
	cfg := &VagrantConfig{}
	cfg.FromMap(config)
	return cfg.Validate()
}

// GenerateInfrastructure is not implemented in Phase 1.
func (p *Provider) GenerateInfrastructure(ctx context.Context, config map[string]interface{}, outputDir string) error {
	return ErrNotImplemented
}

// GetOutputs is not implemented in Phase 1.
func (p *Provider) GetOutputs(ctx context.Context, buildDir string) (*core.InfrastructureOutputs, error) {
	return nil, ErrNotImplemented
}

// GetRequiredFields returns the Vagrant form fields.
func (p *Provider) GetRequiredFields() []core.FormField {
	return GetRequiredFields()
}

// GetDefaultConfig returns the default Vagrant configuration.
func (p *Provider) GetDefaultConfig() map[string]interface{} {
	return GetDefaultConfig()
}
