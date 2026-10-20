package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements the Provider interface for testing.
type mockProvider struct {
	name ProviderType
}

func (m *mockProvider) Name() ProviderType                    { return m.name }
func (m *mockProvider) Validate(map[string]interface{}) error { return nil }
func (m *mockProvider) GenerateInfrastructure(_ context.Context, _ map[string]interface{}, _ string) error {
	return nil
}
func (m *mockProvider) GetOutputs(_ context.Context, _ string) (*InfrastructureOutputs, error) {
	return nil, nil
}
func (m *mockProvider) GetRequiredFields() []FormField           { return nil }
func (m *mockProvider) GetDefaultConfig() map[string]interface{} { return nil }

// mockOrchestrator implements the Orchestrator interface for testing.
type mockOrchestrator struct {
	name OrchestratorType
}

func (m *mockOrchestrator) Name() OrchestratorType                { return m.name }
func (m *mockOrchestrator) Validate(map[string]interface{}) error { return nil }
func (m *mockOrchestrator) GeneratePlaybook(_ context.Context, _ map[string]interface{}, _ string) error {
	return nil
}
func (m *mockOrchestrator) GenerateInventory(_ context.Context, _ *InfrastructureOutputs, _ map[string]interface{}, _ string) error {
	return nil
}
func (m *mockOrchestrator) GetRequiredFields() []FormField           { return nil }
func (m *mockOrchestrator) GetDefaultConfig() map[string]interface{} { return nil }
func (m *mockOrchestrator) GetModules() []Module                     { return nil }

func TestRegistry_RegisterAndGetProvider(t *testing.T) {
	r := NewRegistry()
	r.RegisterProvider(&mockProvider{name: "aws"})

	p, err := r.GetProvider("aws")
	require.NoError(t, err)
	assert.Equal(t, ProviderType("aws"), p.Name())
}

func TestRegistry_GetProviderNotRegistered(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetProvider("azure")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_RegisterAndGetOrchestrator(t *testing.T) {
	r := NewRegistry()
	r.RegisterOrchestrator(&mockOrchestrator{name: "rke2"})

	o, err := r.GetOrchestrator("rke2")
	require.NoError(t, err)
	assert.Equal(t, OrchestratorType("rke2"), o.Name())
}

func TestRegistry_GetOrchestratorNotRegistered(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetOrchestrator("k3s")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_ListProviders(t *testing.T) {
	r := NewRegistry()
	r.RegisterProvider(&mockProvider{name: "aws"})
	r.RegisterProvider(&mockProvider{name: "gcp"})

	providers := r.ListProviders()
	assert.Equal(t, 2, len(providers))
	assert.Contains(t, providers, ProviderType("aws"))
	assert.Contains(t, providers, ProviderType("gcp"))
}

func TestRegistry_ListOrchestrators(t *testing.T) {
	r := NewRegistry()
	r.RegisterOrchestrator(&mockOrchestrator{name: "rke2"})
	r.RegisterOrchestrator(&mockOrchestrator{name: "k3s"})

	orchestrators := r.ListOrchestrators()
	assert.Equal(t, 2, len(orchestrators))
	assert.Contains(t, orchestrators, OrchestratorType("rke2"))
	assert.Contains(t, orchestrators, OrchestratorType("k3s"))
}

func TestRegistry_OverwriteProvider(t *testing.T) {
	r := NewRegistry()
	r.RegisterProvider(&mockProvider{name: "aws"})
	r.RegisterProvider(&mockProvider{name: "aws"})

	providers := r.ListProviders()
	assert.Equal(t, 1, len(providers), "re-registering same name should overwrite, not duplicate")
}
