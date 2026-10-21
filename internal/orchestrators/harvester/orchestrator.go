// Package harvester implements a Harvester HCI orchestrator. It renders the
// per-node Harvester install configs (create/join) used by the ISO auto-install.
//
// It pairs with the local libvirt provider: that provider writes a topology.json
// to the build directory (node names, MACs, IPs, roles), which this orchestrator
// reads to render per-node configs — so the two packages stay decoupled.
package harvester

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/generator"
)

// Orchestrator implements the Harvester orchestrator.
type Orchestrator struct {
	renderer *generator.TemplateRenderer
}

// NewOrchestrator creates a new Harvester orchestrator instance.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{renderer: generator.NewTemplateRenderer()}
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

// nodeData is the template context for a per-node Harvester install config.
type nodeData struct {
	core.HarvesterNode
	Token         string
	Password      string
	VIP           string
	VIPMode       string
	MgmtInterface string
	NTPServers    []string
	ISOURL        string
}

// GeneratePlaybook renders the per-node Harvester install configs (config-<name>.yaml).
// Node 0 gets install mode "create"; the rest get "join" (with server_url+token).
// These are served to the booting VMs by the provider's built-in config server.
func (o *Orchestrator) GeneratePlaybook(ctx context.Context, config map[string]interface{}, outputDir string) error {
	cfg := &HarvesterConfig{}
	cfg.FromMap(config)

	topology, err := core.ReadTopology(outputDir)
	if err != nil {
		return err
	}
	if len(topology.Nodes) == 0 {
		return fmt.Errorf("topology has no nodes")
	}

	tmpl := filepath.Join(getPackageDir(), "templates", "config.yaml.tmpl")
	for _, n := range topology.Nodes {
		nd := nodeData{
			HarvesterNode: n,
			Token:         cfg.ClusterToken,
			Password:      cfg.Password,
			VIP:           cfg.VIP,
			VIPMode:       cfg.VIPMode,
			MgmtInterface: cfg.MgmtInterface,
			NTPServers:    cfg.NTPServers,
			ISOURL:        topology.ISOURL,
		}
		out := filepath.Join(outputDir, "config-"+n.Name+".yaml")
		if err := o.renderer.Render(ctx, tmpl, nd, out); err != nil {
			return fmt.Errorf("render config for %s: %w", n.Name, err)
		}
	}
	return nil
}

// GenerateInventory is a no-op for the ISO-install flow: nodes self-install from
// the ISO and are not configured over SSH/Ansible.
func (o *Orchestrator) GenerateInventory(ctx context.Context, outputs *core.InfrastructureOutputs, config map[string]interface{}, outputDir string) error {
	return nil
}

// GetRequiredFields returns the Harvester form fields.
func (o *Orchestrator) GetRequiredFields() []core.FormField {
	return GetRequiredFields()
}

// GetDefaultConfig returns the default Harvester configuration.
func (o *Orchestrator) GetDefaultConfig() map[string]interface{} {
	return GetDefaultConfig()
}

// GetModules returns the deployment modules (none for Harvester).
func (o *Orchestrator) GetModules() []core.Module {
	return []core.Module{}
}

// getPackageDir returns the package directory relative to the project root.
func getPackageDir() string {
	return "internal/orchestrators/harvester"
}
