package libvirt

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chdirProjectRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	// .../internal/providers/libvirt -> three levels up.
	t.Chdir(filepath.Join(wd, "..", "..", ".."))
}

func TestProvider_Name(t *testing.T) {
	assert.Equal(t, core.ProviderLibvirt, NewProvider().Name())
}

func TestProvider_Validate(t *testing.T) {
	p := NewProvider()
	assert.NoError(t, p.Validate(map[string]interface{}{"cluster_name": "lab"}))
	assert.Error(t, p.Validate(map[string]interface{}{})) // missing cluster_name
}

func TestProvider_ImplementsCustomApply(t *testing.T) {
	var _ core.ProviderWithCustomApply = NewProvider()
}

func TestProvider_GenerateInfrastructure(t *testing.T) {
	chdirProjectRoot(t)
	dir := t.TempDir()

	p := NewProvider()
	cfg := map[string]interface{}{
		"cluster_name": "lab",
		"node_count":   3,
	}
	require.NoError(t, p.GenerateInfrastructure(context.Background(), cfg, dir))

	// libvirt network XML rendered.
	netXML, err := os.ReadFile(filepath.Join(dir, "network.xml"))
	require.NoError(t, err)
	assert.Contains(t, string(netXML), "<name>harvester-lab</name>")
	assert.Contains(t, string(netXML), "192.168.0.1") // gateway

	// Per-node domain XML rendered.
	dom0, err := os.ReadFile(filepath.Join(dir, "domain-harvester-node-0.xml"))
	require.NoError(t, err)
	assert.Contains(t, string(dom0), "<name>lab-harvester-node-0</name>")
	assert.Contains(t, string(dom0), "device='cdrom'")
	_, err = os.Stat(filepath.Join(dir, "domain-harvester-node-2.xml"))
	assert.NoError(t, err)

	// topology.json written and parseable.
	topo, err := core.ReadTopology(dir)
	require.NoError(t, err)
	assert.Len(t, topo.Nodes, 3)
	assert.Equal(t, "create", topo.Nodes[0].Role)
	assert.Equal(t, "join", topo.Nodes[1].Role)
	assert.Equal(t, "192.168.0.30", topo.Nodes[0].IP)
	assert.Equal(t, "192.168.0.1", topo.Gateway)
}

func TestProvider_GenerateInfrastructure_Firmware(t *testing.T) {
	chdirProjectRoot(t)

	// UEFI is the default.
	dir := t.TempDir()
	require.NoError(t, NewProvider().GenerateInfrastructure(context.Background(),
		map[string]interface{}{"cluster_name": "lab", "node_count": 1}, dir))
	uefi, _ := os.ReadFile(filepath.Join(dir, "domain-harvester-node-0.xml"))
	assert.Contains(t, string(uefi), "firmware='efi'")
	assert.Contains(t, string(uefi), "<loader secure='no'/>")

	// BIOS omits the EFI firmware.
	dir2 := t.TempDir()
	require.NoError(t, NewProvider().GenerateInfrastructure(context.Background(),
		map[string]interface{}{"cluster_name": "lab", "node_count": 1, "firmware": "bios"}, dir2))
	bios, _ := os.ReadFile(filepath.Join(dir2, "domain-harvester-node-0.xml"))
	assert.NotContains(t, string(bios), "firmware='efi'")
	assert.NotContains(t, string(bios), "secure='no'")
}

func TestProvider_GenerateInfrastructure_InvalidConfig(t *testing.T) {
	chdirProjectRoot(t)
	dir := t.TempDir()
	// Missing cluster_name -> validation error, no files written.
	err := NewProvider().GenerateInfrastructure(context.Background(), map[string]interface{}{}, dir)
	assert.Error(t, err)
}

func TestProvider_PlannedOutputs(t *testing.T) {
	p := NewProvider()
	out, err := p.PlannedOutputs(map[string]interface{}{"cluster_name": "lab", "node_count": 2})
	require.NoError(t, err)
	assert.Equal(t, []string{"192.168.0.30", "192.168.0.31"}, out.InstanceIPs)
	assert.Equal(t, "192.168.0.1", out.AdditionalOutputs["gateway"])
}

func TestProvider_GetOutputs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, core.WriteTopology(dir, &core.Topology{
		Gateway: "192.168.0.1",
		Subnet:  "192.168.0.0/24",
		Nodes:   []core.HarvesterNode{{Name: "n0", IP: "192.168.0.30"}},
	}))

	out, err := NewProvider().GetOutputs(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"192.168.0.30"}, out.InstanceIPs)
}

func TestGenerateMAC(t *testing.T) {
	// Deterministic: same inputs -> same MAC.
	a := GenerateMAC("lab", 0)
	b := GenerateMAC("lab", 0)
	assert.Equal(t, a, b)

	// KVM-safe prefix.
	assert.True(t, len(a) == 17)
	assert.Equal(t, "52:54:", a[:6])

	// Different inputs -> different MACs.
	assert.NotEqual(t, GenerateMAC("lab", 0), GenerateMAC("lab", 1))
	assert.NotEqual(t, GenerateMAC("lab", 0), GenerateMAC("prod", 0))
}

func TestComputeTopology(t *testing.T) {
	cfg := &Config{ClusterName: "lab"}
	cfg.applyDefaults()
	cfg.NodeCount = 3

	topo, err := ComputeTopology(cfg)
	require.NoError(t, err)
	assert.Equal(t, "192.168.0.1", topo.Gateway)
	require.Len(t, topo.Nodes, 3)
	assert.Equal(t, "harvester-node-0", topo.Nodes[0].Name)
	assert.Equal(t, "create", topo.Nodes[0].Role)
	assert.Equal(t, "join", topo.Nodes[2].Role)
	assert.Equal(t, "192.168.0.32", topo.Nodes[2].IP)
}

func TestComputeTopology_BadSubnet(t *testing.T) {
	cfg := &Config{ClusterName: "lab", NetworkSubnet: "bogus", NodeCount: 1, NodeCPU: 1, NodeMemoryMB: 1, NodeDiskGB: 1, NodePrefix: "n", HarvesterVersion: "v"}
	_, err := ComputeTopology(cfg)
	assert.Error(t, err)
}
