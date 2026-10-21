package harvester

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdirProjectRoot switches CWD to the module root so the CWD-relative template
// paths (getPackageDir) resolve during tests. Restored automatically by t.Chdir.
func chdirProjectRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	// .../internal/orchestrators/harvester -> three levels up.
	t.Chdir(filepath.Join(wd, "..", "..", ".."))
}

func writeTestTopology(t *testing.T, dir string) *core.Topology {
	t.Helper()
	topo := &core.Topology{
		Gateway: "192.168.0.1",
		Subnet:  "192.168.0.0/24",
		ISOURL:  "http://192.168.0.1:8181/harvester.iso",
		Nodes: []core.HarvesterNode{
			{Name: "harvester-node-0", MAC: "52:54:aa:bb:cc:00", IP: "192.168.0.30", Role: "create"},
			{Name: "harvester-node-1", MAC: "52:54:aa:bb:cc:01", IP: "192.168.0.31", Role: "join"},
		},
	}
	require.NoError(t, core.WriteTopology(dir, topo))
	return topo
}

func TestHarvesterOrchestrator_Name(t *testing.T) {
	assert.Equal(t, core.OrchestratorHarvester, NewOrchestrator().Name())
}

func TestHarvesterOrchestrator_GeneratePlaybook(t *testing.T) {
	chdirProjectRoot(t)
	dir := t.TempDir()
	writeTestTopology(t, dir)

	o := NewOrchestrator()
	err := o.GeneratePlaybook(context.Background(), GetDefaultConfig(), dir)
	require.NoError(t, err)

	// Per-node install configs are rendered.
	for _, f := range []string{
		"config-harvester-node-0.yaml",
		"config-harvester-node-1.yaml",
	} {
		_, err := os.Stat(filepath.Join(dir, f))
		assert.NoError(t, err, "expected %s to be generated", f)
	}

	// create node gets mode: create; join node gets a server_url.
	create, _ := os.ReadFile(filepath.Join(dir, "config-harvester-node-0.yaml"))
	assert.Contains(t, string(create), "mode: create")
	assert.NotContains(t, string(create), "server_url")
	assert.Contains(t, string(create), "iso_url: http://192.168.0.1:8181/harvester.iso")

	join, _ := os.ReadFile(filepath.Join(dir, "config-harvester-node-1.yaml"))
	assert.Contains(t, string(join), "mode: join")
	assert.Contains(t, string(join), "server_url: https://192.168.0.50:443")
}

func TestHarvesterOrchestrator_GeneratePlaybook_MissingTopology(t *testing.T) {
	chdirProjectRoot(t)
	dir := t.TempDir()
	err := NewOrchestrator().GeneratePlaybook(context.Background(), GetDefaultConfig(), dir)
	assert.Error(t, err, "should fail without a topology.json")
}

func TestHarvesterOrchestrator_GenerateInventory_NoOp(t *testing.T) {
	// The ISO-install flow configures nodes via the ISO, not SSH/Ansible, so
	// GenerateInventory is a no-op and writes nothing.
	dir := t.TempDir()
	err := NewOrchestrator().GenerateInventory(context.Background(), &core.InfrastructureOutputs{}, GetDefaultConfig(), dir)
	require.NoError(t, err)
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries)
}

func TestHarvesterOrchestrator_Validate(t *testing.T) {
	o := NewOrchestrator()
	assert.NoError(t, o.Validate(GetDefaultConfig()))
	assert.Error(t, o.Validate(map[string]interface{}{
		"harvester_version": "v1.4.1",
		"vip":               "1.2.3.4",
		"vip_mode":          "nonsense",
	}))
}

func TestHarvesterOrchestrator_GetModules(t *testing.T) {
	assert.Empty(t, NewOrchestrator().GetModules())
}
