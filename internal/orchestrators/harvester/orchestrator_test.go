package harvester

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestHarvesterOrchestrator_Name(t *testing.T) {
	o := NewOrchestrator()
	assert.Equal(t, core.OrchestratorHarvester, o.Name())
}

func TestHarvesterOrchestrator_GeneratePlaybook_NotImplemented(t *testing.T) {
	o := NewOrchestrator()
	dir := t.TempDir()

	err := o.GeneratePlaybook(context.Background(), GetDefaultConfig(), dir)
	assert.True(t, errors.Is(err, ErrNotImplemented))

	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries, "scaffold must not write any files")
}

func TestHarvesterOrchestrator_GenerateInventory_NotImplemented(t *testing.T) {
	o := NewOrchestrator()
	dir := t.TempDir()

	err := o.GenerateInventory(context.Background(), &core.InfrastructureOutputs{}, GetDefaultConfig(), dir)
	assert.True(t, errors.Is(err, ErrNotImplemented))

	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries)
}

func TestHarvesterOrchestrator_Validate(t *testing.T) {
	o := NewOrchestrator()
	assert.NoError(t, o.Validate(GetDefaultConfig()))
	assert.Error(t, o.Validate(map[string]interface{}{
		"install_target":  "nonsense",
		"vagrant_backend": "libvirt",
		"node_count":      1,
	}))
}

func TestHarvesterOrchestrator_GetModules(t *testing.T) {
	assert.Nil(t, NewOrchestrator().GetModules())
}
