// Package libvirt implements a local KVM provider that provisions Harvester
// node VMs directly via libvirt (virsh + qemu-img). Each node boots the Harvester
// installer ISO (attached as a CD-ROM) and self-installs unattended; node 0
// creates the cluster and the rest join it automatically.
//
// Vagrant was the original tool here, but its box/SSH model does not fit
// installing an appliance OS from an ISO, so this provider drives libvirt directly.
//
// It implements core.Provider and core.ProviderWithCustomApply (the apply is a
// create-then-join flow that does not fit the OpenTofu pipeline).
package libvirt

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/generator"
)

// projectRoot is where the shared Harvester media cache lives (harvester-isos/).
const projectRoot = "."

// libvirtURI pins virsh to the system libvirt. Harvester needs a real NAT bridge
// network, which the user-session libvirt (qemu:///session) cannot create
// ("Operation not permitted" on bridge creation). The invoking user must be in
// the "libvirt" group (or root).
const libvirtURI = "qemu:///system"

// runVirsh runs a virsh command against the system libvirt, streaming output.
func runVirsh(ctx context.Context, dir string, logWriter io.Writer, args ...string) error {
	return runStreaming(ctx, dir, logWriter, "virsh", append([]string{"-c", libvirtURI}, args...)...)
}

// VIP polling tuning for the Harvester cluster bring-up.
const (
	vipPollAttempts = 90
	vipPollInterval = 30 * time.Second
	vipHTTPTimeout  = 10 * time.Second
)

// Provider implements the local libvirt/KVM provider.
type Provider struct {
	renderer *generator.TemplateRenderer
}

// NewProvider creates a new provider instance.
func NewProvider() *Provider {
	return &Provider{renderer: generator.NewTemplateRenderer()}
}

// Name returns the provider type.
func (p *Provider) Name() core.ProviderType {
	return core.ProviderLibvirt
}

// Validate validates the configuration (config only). Host prerequisites
// (virsh/qemu-img/libvirt/KVM) are checked at apply time in CustomApply.
func (p *Provider) Validate(config map[string]interface{}) error {
	cfg := &Config{}
	cfg.FromMap(config)
	return cfg.Validate()
}

// networkData is the template context for network.xml.tmpl.
type networkData struct {
	ClusterName string
	Gateway     string
	DHCPStart   string
	DHCPEnd     string
	Nodes       []core.HarvesterNode
}

// domainData is the template context for domain.xml.tmpl.
type domainData struct {
	Name        string // libvirt domain name (cluster-prefixed, unique on host)
	ClusterName string
	Role        string
	MemoryMB    int
	CPUs        int
	DiskPath    string // absolute path to the blank qcow2 install disk
	ISOPath     string // absolute path to the auto-install ISO (CD-ROM)
	MAC         string
	Firmware    string // "uefi" or "bios"
}

// domainName returns the unique libvirt domain name for a node.
func domainName(clusterName, nodeName string) string {
	return clusterName + "-" + nodeName
}

// GenerateInfrastructure renders the libvirt network + per-node domain XML and
// writes the topology.json consumed by the Harvester orchestrator.
func (p *Provider) GenerateInfrastructure(ctx context.Context, config map[string]interface{}, outputDir string) error {
	cfg := &Config{}
	cfg.FromMap(config)
	if err := cfg.Validate(); err != nil {
		return err
	}

	topology, err := ComputeTopology(cfg)
	if err != nil {
		return err
	}
	if err := core.WriteTopology(outputDir, topology); err != nil {
		return err
	}

	base, err := subnetBase(cfg.NetworkSubnet)
	if err != nil {
		return err
	}

	// The auto-install ISO path is deterministic from the config URL, so we can
	// reference it in the domain XML now even though CustomApply builds it later.
	mediaDir, err := filepath.Abs(cfg.MediaDir(projectRoot))
	if err != nil {
		return err
	}
	isoPath := AutoInstallISOPath(mediaDir, cfg.HarvesterVersion, configURL(topology.Gateway))

	absOut, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}

	// Network XML.
	netData := networkData{
		ClusterName: cfg.ClusterName,
		Gateway:     topology.Gateway,
		DHCPStart:   base + ".100",
		DHCPEnd:     base + ".200",
		Nodes:       topology.Nodes,
	}
	if err := p.renderer.Render(ctx, p.tmpl("network.xml.tmpl"), netData, filepath.Join(outputDir, "network.xml")); err != nil {
		return err
	}

	// Per-node domain XML.
	for _, n := range topology.Nodes {
		d := domainData{
			Name:        domainName(cfg.ClusterName, n.Name),
			ClusterName: cfg.ClusterName,
			Role:        n.Role,
			MemoryMB:    cfg.NodeMemoryMB,
			CPUs:        cfg.NodeCPU,
			DiskPath:    filepath.Join(absOut, n.Name+".qcow2"),
			ISOPath:     isoPath,
			MAC:         n.MAC,
			Firmware:    cfg.Firmware,
		}
		if err := p.renderer.Render(ctx, p.tmpl("domain.xml.tmpl"), d, filepath.Join(outputDir, "domain-"+n.Name+".xml")); err != nil {
			return err
		}
	}
	return nil
}

// PlannedOutputs returns the deterministic outputs derived from config alone.
func (p *Provider) PlannedOutputs(config map[string]interface{}) (*core.InfrastructureOutputs, error) {
	cfg := &Config{}
	cfg.FromMap(config)
	topology, err := ComputeTopology(cfg)
	if err != nil {
		return nil, err
	}
	return topologyOutputs(topology), nil
}

// GetOutputs reads the planned topology from the build dir after provisioning.
func (p *Provider) GetOutputs(ctx context.Context, buildDir string) (*core.InfrastructureOutputs, error) {
	topology, err := core.ReadTopology(buildDir)
	if err != nil {
		return nil, err
	}
	return topologyOutputs(topology), nil
}

// topologyOutputs maps a topology to InfrastructureOutputs.
func topologyOutputs(t *core.Topology) *core.InfrastructureOutputs {
	ips := make([]string, 0, len(t.Nodes))
	for _, n := range t.Nodes {
		ips = append(ips, n.IP)
	}
	return &core.InfrastructureOutputs{
		InstanceIPs:       ips,
		InstanceDNSNames:  []string{},
		PrivateIPs:        []string{},
		AdditionalOutputs: map[string]interface{}{"gateway": t.Gateway},
	}
}

// configURL is the URL the installer fetches its config from (served by the
// built-in config server on the host, at the libvirt network gateway).
func configURL(gateway string) string {
	return fmt.Sprintf("http://%s:%d/config", gateway, ConfigServerPort)
}

// CustomApply builds the auto-install ISO, starts the config server, and boots
// the node VMs (node 0 creates the cluster, the rest join), streaming progress.
func (p *Provider) CustomApply(ctx context.Context, config map[string]interface{}, buildDir string, logWriter io.Writer) (*core.InfrastructureOutputs, error) {
	if err := CheckPrerequisites(); err != nil {
		return nil, err
	}

	cfg := &Config{}
	cfg.FromMap(config)
	topology, err := ComputeTopology(cfg)
	if err != nil {
		return nil, err
	}

	mediaDir, err := filepath.Abs(cfg.MediaDir(projectRoot))
	if err != nil {
		return nil, err
	}

	// 1. Ensure the installer ISO is present (download if missing / not user-supplied).
	fmt.Fprintf(logWriter, "Ensuring Harvester %s ISO is available...\n", cfg.HarvesterVersion)
	if err := EnsureISOInDir(ctx, mediaDir, cfg.HarvesterVersion, logWriter); err != nil {
		return nil, err
	}
	srcISO := filepath.Join(mediaDir, ISOName(cfg.HarvesterVersion))
	if cfg.ISOPath != "" {
		srcISO = cfg.ISOPath
	}

	// 2. Build the unattended auto-install ISO (patched GRUB + config_url).
	url := configURL(topology.Gateway)
	autoISO := AutoInstallISOPath(mediaDir, cfg.HarvesterVersion, url)
	if err := RepackAutoInstallISO(ctx, srcISO, autoISO, url, logWriter); err != nil {
		return nil, err
	}

	// 3. Define + start the isolated libvirt network.
	if err := p.ensureNetwork(ctx, buildDir, cfg.ClusterName, logWriter); err != nil {
		return nil, err
	}

	// 4. Start the built-in config server (serves per-node install config by IP).
	byIP := map[string]string{}
	for _, n := range topology.Nodes {
		byIP[n.IP] = filepath.Join(buildDir, "config-"+n.Name+".yaml")
	}
	server, err := StartConfigServer(fmt.Sprintf("0.0.0.0:%d", ConfigServerPort), byIP, srcISO, logWriter)
	if err != nil {
		return nil, err
	}
	defer server.Close()

	// 5. Boot node 0 (create) first and wait for the cluster VIP.
	vip, _ := config["vip"].(string)
	for i, n := range topology.Nodes {
		if err := p.bootNode(ctx, buildDir, cfg, n, logWriter); err != nil {
			return nil, err
		}
		// After the first (create) node, wait for the cluster to be up before
		// booting the joiners, otherwise they have nothing to join.
		if i == 0 && len(topology.Nodes) > 1 && vip != "" {
			fmt.Fprintln(logWriter, "Waiting for the cluster to form before joining more nodes...")
			if err := waitForVIP(ctx, vip, logWriter); err != nil {
				return nil, err
			}
		}
	}

	if vip != "" {
		if err := waitForVIP(ctx, vip, logWriter); err != nil {
			return nil, err
		}
	} else {
		fmt.Fprintln(logWriter, "No VIP configured; skipping cluster readiness wait.")
	}

	outputs := topologyOutputs(topology)
	outputs.AdditionalOutputs["vip"] = vip
	return outputs, nil
}

// bootNode creates the blank install disk (if needed), defines the domain and
// starts it.
func (p *Provider) bootNode(ctx context.Context, buildDir string, cfg *Config, n core.HarvesterNode, logWriter io.Writer) error {
	disk, err := filepath.Abs(filepath.Join(buildDir, n.Name+".qcow2"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(disk); os.IsNotExist(err) {
		fmt.Fprintf(logWriter, "Creating %dG disk for %s...\n", cfg.NodeDiskGB, n.Name)
		if err := runStreaming(ctx, buildDir, logWriter, "qemu-img", "create", "-f", "qcow2", disk, fmt.Sprintf("%dG", cfg.NodeDiskGB)); err != nil {
			return fmt.Errorf("create disk for %s: %w", n.Name, err)
		}
	}

	dom := domainName(cfg.ClusterName, n.Name)
	domXML, err := filepath.Abs(filepath.Join(buildDir, "domain-"+n.Name+".xml"))
	if err != nil {
		return err
	}
	fmt.Fprintf(logWriter, "Defining and starting VM %s (%s)...\n", dom, n.Role)
	if err := runVirsh(ctx, buildDir, logWriter, "define", domXML); err != nil {
		return fmt.Errorf("virsh define %s: %w", dom, err)
	}
	if err := runVirsh(ctx, buildDir, logWriter, "start", dom); err != nil {
		return fmt.Errorf("virsh start %s: %w", dom, err)
	}
	return nil
}

// ensureNetwork defines and starts the cluster's libvirt network if absent.
func (p *Provider) ensureNetwork(ctx context.Context, buildDir, clusterName string, logWriter io.Writer) error {
	net := "harvester-" + clusterName
	// Already defined + active?
	if out, err := commandRunner("virsh", "-c", libvirtURI, "net-info", net); err == nil && hasActiveYes(string(out)) {
		return nil
	}
	netXML, err := filepath.Abs(filepath.Join(buildDir, "network.xml"))
	if err != nil {
		return err
	}
	fmt.Fprintf(logWriter, "Defining libvirt network %s...\n", net)
	if err := runVirsh(ctx, buildDir, logWriter, "net-define", netXML); err != nil {
		// Ignore "already exists" style errors; continue to start.
		fmt.Fprintf(logWriter, "net-define note: %v\n", err)
	}
	_ = runVirsh(ctx, buildDir, logWriter, "net-autostart", net)
	if err := runVirsh(ctx, buildDir, logWriter, "net-start", net); err != nil {
		return fmt.Errorf("virsh net-start %s: %w", net, err)
	}
	return nil
}

// waitForVIP polls https://<vip>:443 until the Harvester cluster responds.
func waitForVIP(ctx context.Context, vip string, logWriter io.Writer) error {
	client := &http.Client{
		Timeout:   vipHTTPTimeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	url := fmt.Sprintf("https://%s:443", vip)
	start := time.Now()

	for attempt := 1; attempt <= vipPollAttempts; attempt++ {
		elapsed := time.Since(start).Round(time.Second)
		fmt.Fprintf(logWriter, "[harvester-wait] attempt=%d elapsed=%s\n", attempt, elapsed)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			fmt.Fprintf(logWriter, "[harvester-ready] vip=%s\n", vip)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(vipPollInterval):
		}
	}
	return fmt.Errorf("harvester cluster did not come up at %s after %s", url, time.Since(start).Round(time.Second))
}

// runStreaming runs a command in dir, streaming stdout+stderr to logWriter.
func runStreaming(ctx context.Context, dir string, logWriter io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	return cmd.Run()
}

// Teardown destroys the cluster's VMs, network and install disks (best effort).
// Each step is tolerant of already-absent resources so a partial deploy still
// cleans up. It does NOT delete the shared media cache or the auto-install ISOs.
func Teardown(ctx context.Context, clusterName, buildDir string, logWriter io.Writer) error {
	var nodeNames []string
	if t, err := core.ReadTopology(buildDir); err == nil {
		for _, n := range t.Nodes {
			nodeNames = append(nodeNames, n.Name)
		}
	}

	for _, nn := range nodeNames {
		dom := domainName(clusterName, nn)
		_ = runVirsh(ctx, buildDir, logWriter, "destroy", dom)
		// --nvram removes the per-VM UEFI vars file too.
		_ = runVirsh(ctx, buildDir, logWriter, "undefine", "--nvram", dom)
		if disk, err := filepath.Abs(filepath.Join(buildDir, nn+".qcow2")); err == nil {
			if err := os.Remove(disk); err == nil {
				fmt.Fprintf(logWriter, "removed disk %s\n", disk)
			}
		}
	}

	net := "harvester-" + clusterName
	_ = runVirsh(ctx, buildDir, logWriter, "net-destroy", net)
	_ = runVirsh(ctx, buildDir, logWriter, "net-undefine", net)
	return nil
}

// GetRequiredFields returns the provider form fields.
func (p *Provider) GetRequiredFields() []core.FormField {
	return GetRequiredFields()
}

// GetDefaultConfig returns the default provider configuration.
func (p *Provider) GetDefaultConfig() map[string]interface{} {
	return GetDefaultConfig()
}

// tmpl returns the path to a template file.
func (p *Provider) tmpl(name string) string {
	return filepath.Join(getPackageDir(), "templates", name)
}

// getPackageDir returns the package directory, relative to the project root.
func getPackageDir() string {
	return "internal/providers/libvirt"
}

// hasActiveYes reports whether `virsh net-info` output marks the network active
// (a line like "Active:         yes").
func hasActiveYes(netInfo string) bool {
	for _, line := range strings.Split(netInfo, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Active:") {
			return strings.Contains(strings.ToLower(line), "yes")
		}
	}
	return false
}
