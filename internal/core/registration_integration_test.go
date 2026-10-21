package core_test

import (
	"testing"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/orchestrators/harvester"
	"github.com/Felipalds/rancher-tui/internal/providers/libvirt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegister_HarvesterAndVagrant verifies the new scaffolds register and
// resolve through a fresh registry (isolated from the global one).
func TestRegister_HarvesterAndVagrant(t *testing.T) {
	reg := core.NewRegistry()
	reg.RegisterOrchestrator(harvester.NewOrchestrator())
	reg.RegisterProvider(libvirt.NewProvider())

	o, err := reg.GetOrchestrator(core.OrchestratorHarvester)
	require.NoError(t, err)
	assert.Equal(t, core.OrchestratorHarvester, o.Name())
	assert.Contains(t, reg.ListOrchestrators(), core.OrchestratorHarvester)

	p, err := reg.GetProvider(core.ProviderLibvirt)
	require.NoError(t, err)
	assert.Equal(t, core.ProviderLibvirt, p.Name())
	assert.Contains(t, reg.ListProviders(), core.ProviderLibvirt)
}
