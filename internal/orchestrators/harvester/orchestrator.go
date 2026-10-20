// Package harvester scaffolds a Harvester HCI orchestrator.
//
// Phase 1 provides the config, validation, form fields and registry wiring so
// Harvester is selectable in the UI, but playbook/inventory generation is not
// implemented yet (returns ErrNotImplemented). Full cloud and Vagrant/libvirt
// install flows arrive in a later phase — see feats/2-harvester-vagrant.md.
package harvester

import (
	"context"
	"errors"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// ErrNotImplemented is returned by generation methods that are not yet built.
var ErrNotImplemented = errors.New("harvester: not implemented in Phase 1")

// Orchestrator implements the Harvester orchestrator (scaffold).
type Orchestrator struct{}

// NewOrchestrator creates a new Harvester orchestrator instance.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{}
}

// Name returns the orchestrator type.
func (o *Orchestrator) Name() core.OrchestratorType {
	return core.OrchestratorHarvester
}

// Validate validates the Harvester configuration.
func (o *Orchestrator) Validate(config map[string]interface{}) error {
	cfg := &HarvesterConfig{}
	cfg.FromMap(config)
	return cfg.Validate()
}

// GeneratePlaybook is not implemented in Phase 1.
func (o *Orchestrator) GeneratePlaybook(ctx context.Context, config map[string]interface{}, outputDir string) error {
	return ErrNotImplemented
}

// GenerateInventory is not implemented in Phase 1.
func (o *Orchestrator) GenerateInventory(ctx context.Context, outputs *core.InfrastructureOutputs, config map[string]interface{}, outputDir string) error {
	return ErrNotImplemented
}

// GetRequiredFields returns the Harvester form fields.
func (o *Orchestrator) GetRequiredFields() []core.FormField {
	return GetRequiredFields()
}

// GetDefaultConfig returns the default Harvester configuration.
func (o *Orchestrator) GetDefaultConfig() map[string]interface{} {
	return GetDefaultConfig()
}

// GetModules returns the deployment modules (none yet).
func (o *Orchestrator) GetModules() []core.Module {
	return nil
}
