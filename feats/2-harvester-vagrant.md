# Feature: Harvester Cluster Deployment via Vagrant + iPXE

## Summary

Add a new **provider** (`vagrant`) and **orchestrator** (`harvester`) to rancher-corral, enabling automated deployment of [Harvester](https://harvesterhci.io/) HCI clusters on the local machine using Vagrant with libvirt/KVM. This follows the iPXE-based approach from [harvester/ipxe-examples](https://github.com/harvester/ipxe-examples), adapted to fit corral's Provider/Orchestrator architecture.

Instead of provisioning EC2 instances (AWS provider) and installing RKE2/K3s (orchestrators), this creates local VMs via Vagrant: a PXE server VM that serves boot media, and N Harvester node VMs that PXE-boot into a fully automated Harvester installation.

---

## Motivation

Harvester is SUSE's HCI solution built on Kubernetes. Testing Rancher + Harvester integration requires a running Harvester cluster, which is painful to set up manually. The `ipxe-examples` repo automates this via Vagrant, but:

1. It's a standalone project — not integrated into corral's workflow
2. It requires manual editing of `settings.yml` and running Ansible separately
3. It doesn't track cluster state, provide a TUI, or manage lifecycle (create/delete)

By integrating Harvester as a first-class provider+orchestrator pair, corral users can spin up Harvester clusters with the same TUI workflow they use for RKE2/K3s on AWS.

**Use cases:**
- Local Harvester dev/test environments
- Rancher + Harvester integration testing
- CI/CD pipelines for Harvester-related features
- Air-gapped Harvester lab environments

---

## Architecture Decision: Provider + Orchestrator Split

### Why Vagrant is a Provider (not just an Orchestrator)

The Vagrant/Harvester deployment doesn't fit neatly into the existing split where:
- **Provider** = infrastructure (VMs, networking)
- **Orchestrator** = Kubernetes distribution (RKE2/K3s install)

With Harvester, the "infrastructure" and "Kubernetes" are tightly coupled — Harvester IS the OS, the hypervisor, AND the Kubernetes distribution. However, we can still split responsibilities:

| Component | Role | Justification |
|---|---|---|
| **Vagrant Provider** | Creates VMs (PXE server + Harvester nodes) via Vagrantfile | Manages libvirt/KVM VMs, networking, disk allocation — pure infrastructure |
| **Harvester Orchestrator** | Generates PXE server configs, Harvester install configs, iPXE scripts | Manages Harvester-specific config: install mode, VIP, cluster token, node roles |

This preserves the architecture's separation of concerns and allows potential reuse (e.g., a future "bare-metal" provider could pair with the Harvester orchestrator).

### Alternative Considered: Single "Harvester" Provider

We could bundle everything into one provider and skip the orchestrator. Rejected because:
- Breaks the registry pattern (every deployment needs both a provider and orchestrator)
- Loses the ability to potentially deploy Harvester on other infrastructure (bare metal, Equinix)
- Makes the single provider too large and hard to test

---

## Design Decisions

| Decision | Choice | Reason |
|---|---|---|
| Virtualization backend | libvirt/KVM via vagrant-libvirt | Same as ipxe-examples; best Linux VM performance; nested virtualization support |
| PXE boot approach | Dedicated PXE server VM (Debian) with DHCP+TFTP+HTTP | Proven approach from ipxe-examples; isolated from host networking |
| IaC tool | Vagrantfile (Ruby DSL) generated from Go template | Vagrant manages VM lifecycle; no need for OpenTofu here |
| Config management | Ansible (reuse ipxe-examples roles) | PXE server setup is complex; Ansible roles already exist and are battle-tested |
| Harvester media source | Download from releases.rancher.com (configurable URL) | Same as ipxe-examples; supports master/stable/specific versions |
| Network mode | Isolated libvirt network (no host DHCP conflict) | PXE server runs its own DHCP; must not conflict with host network |
| Node roles | Configurable: default, management, witness, worker | Harvester supports multiple roles; expose in TUI |
| VIP mode | DHCP (default) or static | Harvester cluster HA requires a VIP |
| Cluster token | Auto-generated or user-specified | Simplifies creation; advanced users can set their own |
| Default node count | 3 (minimum for HA) | Harvester recommends 3 nodes for production-like testing |
| Rancher integration | Optional (same as existing RKE2/K3s flow) | Can deploy Rancher on a separate K3s VM or import Harvester into existing Rancher |

---

## New Package Structure

```
internal/
├── providers/
│   ├── aws/              # Existing
│   └── vagrant/          # NEW
│       ├── provider.go       # Vagrant Provider implementation
│       ├── config.go         # VagrantConfig struct + FromMap()
│       ├── validator.go      # Validation (vagrant, libvirt, KVM checks)
│       └── templates/
│           └── Vagrantfile.tmpl   # Vagrantfile template
│
├── orchestrators/
│   ├── rke2/             # Existing
│   ├── k3s/              # Existing
│   └── harvester/        # NEW
│       ├── orchestrator.go   # Harvester Orchestrator implementation
│       ├── config.go         # HarvesterConfig struct + FromMap()
│       └── templates/
│           ├── settings.yml.tmpl         # ipxe-examples settings.yml
│           ├── config-create.yaml.tmpl   # Harvester install config (create mode)
│           ├── config-join.yaml.tmpl     # Harvester install config (join mode)
│           ├── ipxe-create.tmpl          # iPXE boot script (first node)
│           ├── ipxe-join.tmpl            # iPXE boot script (join nodes)
│           ├── dhcpd.conf.tmpl           # ISC-DHCP config
│           └── setup-pxe.yml.tmpl        # Ansible playbook for PXE server
```

---

## Provider: Vagrant

### Interface Implementation

```go
type VagrantProvider struct{}

func (p *VagrantProvider) Name() core.ProviderType { return "vagrant" }

func (p *VagrantProvider) Validate(config map[string]interface{}) error {
    // Check: vagrant binary exists
    // Check: vagrant-libvirt plugin installed
    // Check: libvirtd running
    // Check: KVM available (/dev/kvm)
    // Check: sufficient resources (RAM, disk)
}

func (p *VagrantProvider) GenerateInfrastructure(ctx context.Context, config map[string]interface{}, outputDir string) error {
    // Render Vagrantfile.tmpl → Vagrantfile
    // Defines: pxe_server VM + N harvester-node VMs
}

func (p *VagrantProvider) GetOutputs(ctx context.Context, buildDir string) (*core.InfrastructureOutputs, error) {
    // Run: vagrant status --machine-readable
    // Parse: node IPs from vagrant ssh-config or libvirt network
    // Return: InfrastructureOutputs with PXE server IP + node IPs
}

func (p *VagrantProvider) GetRequiredFields() []core.FormField {
    return []core.FormField{
        {Name: "node_count", Label: "Number of Harvester Nodes", Type: core.FieldText, Default: "3"},
        {Name: "node_cpu", Label: "CPUs per Node", Type: core.FieldText, Default: "8"},
        {Name: "node_memory", Label: "Memory per Node (MB)", Type: core.FieldText, Default: "16384"},
        {Name: "node_disk", Label: "Disk per Node", Type: core.FieldText, Default: "500G"},
        {Name: "network_subnet", Label: "Network Subnet", Type: core.FieldText, Default: "192.168.0.0/24"},
        {Name: "pxe_server_ip", Label: "PXE Server IP", Type: core.FieldText, Default: "192.168.0.254"},
    }
}
```

### Vagrantfile Template (Simplified)

The generated Vagrantfile creates:
1. A libvirt network named `harvester-<cluster_name>` with DHCP disabled
2. A PXE server VM (Debian 11) with static IP
3. N Harvester node VMs with PXE boot enabled, no OS, assigned MAC addresses

### Workflow Differences from AWS

| Step | AWS Provider | Vagrant Provider |
|---|---|---|
| Generate infra | Renders `main.tf` | Renders `Vagrantfile` |
| Init | `tofu init` | (no-op, or `vagrant validate`) |
| Apply | `tofu apply` | `vagrant up pxe_server` (just the PXE server first) |
| Get outputs | `tofu output -json` | `vagrant ssh-config` + libvirt network inspection |
| Teardown | `tofu destroy` | `vagrant destroy -f` |

---

## Orchestrator: Harvester

### Interface Implementation

```go
type HarvesterOrchestrator struct{}

func (o *HarvesterOrchestrator) Name() core.OrchestratorType { return "harvester" }

func (o *HarvesterOrchestrator) Validate(config map[string]interface{}) error {
    // Check: harvester_version is set
    // Check: VIP is within subnet range
    // Check: node_count >= 1
}

func (o *HarvesterOrchestrator) GeneratePlaybook(ctx context.Context, config map[string]interface{}, outputDir string) error {
    // Render: setup-pxe.yml.tmpl → setup-pxe.yml (Ansible playbook)
    //   - Installs DHCP, TFTP, HTTP on PXE server
    //   - Downloads Harvester media (kernel, initrd, rootfs, ISO)
    //   - Generates per-node iPXE scripts and config files
    //
    // Unlike RKE2/K3s where the playbook installs K8s on the nodes,
    // here the playbook configures the PXE server, and the nodes
    // self-install via iPXE boot.
}

func (o *HarvesterOrchestrator) GenerateInventory(ctx context.Context, outputs *core.InfrastructureOutputs, config map[string]interface{}, outputDir string) error {
    // Render: hosts.ini with [pxe_server] group only
    // The PXE server is the only host Ansible touches directly;
    // Harvester nodes are configured via PXE boot, not SSH.
}

func (o *HarvesterOrchestrator) GetRequiredFields() []core.FormField {
    return []core.FormField{
        {Name: "harvester_version", Label: "Harvester Version", Type: core.FieldText, Default: "v1.4.1"},
        {Name: "cluster_token", Label: "Cluster Token", Type: core.FieldText, Default: "token"},
        {Name: "password", Label: "Node Password", Type: core.FieldText, Default: "p@ssword"},
        {Name: "vip", Label: "Cluster VIP", Type: core.FieldText, Default: "192.168.0.131"},
        {Name: "vip_mode", Label: "VIP Mode", Type: core.FieldSelect, Options: []string{"DHCP", "static"}, Default: "DHCP"},
        {Name: "deploy_rancher", Label: "Deploy Rancher", Type: core.FieldSelect, Options: []string{"No", "Yes"}, Default: "No"},
        {Name: "ntp_servers", Label: "NTP Servers", Type: core.FieldText, Default: "0.suse.pool.ntp.org"},
    }
}
```

### Key Difference: Two-Phase Deployment

Unlike RKE2/K3s where the workflow is:
```
Create VMs → SSH into VMs → Install K8s via Ansible
```

Harvester's flow is:
```
Create PXE Server VM → Configure PXE Server via Ansible → Boot Harvester VMs → VMs PXE-boot and self-install
```

This means the **ModularRunner needs modification** to support a two-phase `Apply`:

1. **Phase 1**: `vagrant up pxe_server` — boot and provision PXE server
2. **Run Ansible** on PXE server (configure DHCP, TFTP, HTTP, download media)
3. **Phase 2**: `vagrant up harvester-node-0 harvester-node-1 ...` — boot nodes (they PXE install)
4. **Wait** for Harvester cluster VIP to respond on HTTPS

---

## Modified Workflow for Harvester

The existing 9-step ModularRunner pipeline needs adaptation. Two approaches:

### Option A: Override Steps in Provider/Orchestrator (Recommended)

Add an optional `CustomApply` method to the Provider interface:

```go
type ProviderWithCustomApply interface {
    Provider
    // CustomApply replaces the standard init+apply+getOutputs+ansible flow
    CustomApply(ctx context.Context, config map[string]interface{}, buildDir string, logWriter io.Writer) (*InfrastructureOutputs, error)
}
```

The Vagrant provider implements this to handle the two-phase boot:

```go
func (p *VagrantProvider) CustomApply(ctx, config, buildDir, logWriter) (*InfrastructureOutputs, error) {
    // 1. vagrant up pxe_server
    // 2. ansible-playbook setup-pxe.yml (configure PXE server)
    // 3. vagrant up harvester-node-0 harvester-node-1 ...
    // 4. Wait for Harvester VIP HTTPS (retries with timeout)
    // 5. Return outputs (VIP, node IPs)
}
```

The ModularRunner checks for this interface and delegates when available, skipping the standard init/apply/ansible steps.

### Option B: Multi-Step Pipeline

Make the runner pipeline configurable with provider-specific steps. More flexible but higher complexity. Not recommended for the first implementation.

---

## Harvester Media URLs

Based on ipxe-examples, media is downloaded from `releases.rancher.com`:

**Decision: local ISO cache.** Create `harvester-isos/<version>/` at the project root (added to `.gitignore`). At the start of `CustomApply`, call `EnsureISO(version)` which downloads all four artifacts (iso, vmlinuz, initrd, rootfs) if they don't exist, streaming download progress to the log viewer. The Vagrantfile mounts `harvester-isos/<version>/` into the PXE server VM as a synced folder at `/harvester-media`, so nginx serves local files instead of downloading during install.

```
https://releases.rancher.com/harvester/{version}/harvester-{version}-amd64.iso
https://releases.rancher.com/harvester/{version}/harvester-{version}-vmlinuz-amd64
https://releases.rancher.com/harvester/{version}/harvester-{version}-initrd-amd64
https://releases.rancher.com/harvester/{version}/harvester-{version}-rootfs-amd64.squashfs
```

For `master` builds:
```
https://releases.rancher.com/harvester/master/harvester-master-amd64.iso
```

The orchestrator config specifies the version, and URLs are constructed automatically. Users can override with custom URLs for local/airgap media.

---

## Network Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Host Machine (KVM/libvirt)                                 │
├─────────────────────────────────────────────────────────────┤
│  libvirt network: "harvester-<cluster_name>"                │
│  Subnet: 192.168.0.0/24 (configurable)                     │
│  DHCP: disabled (PXE server provides)                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────────┐                                   │
│  │ pxe_server           │ .254                              │
│  │ Debian 11            │                                   │
│  │ DHCP + TFTP + HTTP   │                                   │
│  │ Harvester media      │                                   │
│  └──────────────────────┘                                   │
│                                                             │
│  ┌──────────────────────┐                                   │
│  │ harvester-node-0     │ .30  (create mode)                │
│  │ 8 CPU / 16GB / 500G  │                                   │
│  └──────────────────────┘                                   │
│                                                             │
│  ┌──────────────────────┐                                   │
│  │ harvester-node-1     │ .31  (join mode)                  │
│  │ 8 CPU / 16GB / 500G  │                                   │
│  └──────────────────────┘                                   │
│                                                             │
│  ┌──────────────────────┐                                   │
│  │ harvester-node-2     │ .32  (join mode)                  │
│  │ 8 CPU / 16GB / 500G  │                                   │
│  └──────────────────────┘                                   │
│                                                             │
│  VIP: .131 (floating, managed by Harvester)                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## TUI Changes

### Create Form — Provider Selection

Add `vagrant` to the provider select field. When selected, the form dynamically shows Vagrant-specific fields instead of AWS fields:

**AWS fields (hidden when vagrant selected):**
- Credentials, Region, Subnet ID, Security Group ID, OS Image, Instance Type

**Vagrant fields (shown when vagrant selected):**
- CPUs per Node, Memory per Node, Disk per Node, Network Subnet, PXE Server IP

**Common fields (always shown):**
- Cluster Name, Node Count

### Create Form — Orchestrator Selection

Add `harvester` to the Kubernetes Distribution select. When selected, show Harvester-specific fields instead of RKE2/K3s fields:

**RKE2/K3s fields (hidden when harvester selected):**
- K8s Version, Deploy Rancher, Rancher Version, Bootstrap Password, Image Tag, Debug

**Harvester fields (shown when harvester selected):**
- Harvester Version, Cluster Token, Node Password, VIP, VIP Mode, NTP Servers
- Deploy Rancher (optional — deploys on separate K3s VM)

### Cluster List

Harvester clusters show in the same list with:
- Provider: `vagrant` (instead of `aws`)
- Distribution: `harvester` (instead of `rke2`/`k3s`)
- URL: Harvester dashboard at `https://<VIP>:443`

---

## Config Example

```yaml
clusters:
  my-harvester-lab:
    provider:
      type: vagrant
      node_cpu: 8
      node_memory: 16384
      node_disk: "500G"
      network_subnet: "192.168.0.0/24"
      pxe_server_ip: "192.168.0.254"
    kubernetes:
      distribution: harvester
      version: "v1.4.1"
    harvester:
      cluster_token: "my-token"
      password: "p@ssword"
      vip: "192.168.0.131"
      vip_mode: "DHCP"
      ntp_servers:
        - "0.suse.pool.ntp.org"
    ssh:
      key_name: ""
      private_key_path: "~/.ssh/id_rsa"
      user: "rancher"
    cluster:
      name: "my-harvester-lab"
      node_count: 3
      node_prefix: "harvester-node"
    status: "running"
    rancher_url: "https://192.168.0.131"
    instance_ips:
      - "192.168.0.30"
      - "192.168.0.31"
      - "192.168.0.32"
```

---

## Files to Create

| File | Purpose |
|---|---|
| `internal/providers/vagrant/provider.go` | Vagrant Provider with CustomApply |
| `internal/providers/vagrant/config.go` | VagrantConfig struct + FromMap() |
| `internal/providers/vagrant/validator.go` | Prerequisite checks (vagrant, libvirt, KVM) |
| `internal/providers/vagrant/templates/Vagrantfile.tmpl` | Vagrantfile generation |
| `internal/orchestrators/harvester/orchestrator.go` | Harvester Orchestrator |
| `internal/orchestrators/harvester/config.go` | HarvesterConfig struct + FromMap() |
| `internal/orchestrators/harvester/templates/settings.yml.tmpl` | iPXE-examples compatible settings |
| `internal/orchestrators/harvester/templates/config-create.yaml.tmpl` | Create-mode install config |
| `internal/orchestrators/harvester/templates/config-join.yaml.tmpl` | Join-mode install config |
| `internal/orchestrators/harvester/templates/ipxe-create.tmpl` | iPXE boot script (create) |
| `internal/orchestrators/harvester/templates/ipxe-join.tmpl` | iPXE boot script (join) |
| `internal/orchestrators/harvester/templates/dhcpd.conf.tmpl` | DHCP server config |
| `internal/orchestrators/harvester/templates/setup-pxe.yml.tmpl` | Ansible playbook for PXE server |

## Files to Modify

| File | Change |
|---|---|
| `main.go` | Register `vagrant` provider and `harvester` orchestrator in `init()` |
| `internal/core/interfaces.go` | Add `ProviderWithCustomApply` interface |
| `internal/workflow/runner_new.go` | Check for `CustomApply` interface; delegate when available |
| `internal/config/clusters.go` | Add `HarvesterSection` to `ClusterConfig`; update `ToModernConfig`/`FromModernConfig` |
| `internal/tui/views/createform.go` | Add vagrant/harvester to provider/orchestrator selects; dynamic field visibility |
| `internal/tui/views/messages.go` | Add any new message types if needed |

---

## Implementation Order

1. **Harvester Orchestrator** — templates + config + orchestrator.go (can be tested independently)
2. **Vagrant Provider** — Vagrantfile template + provider.go + validator.go
3. **Core changes** — `ProviderWithCustomApply` interface + ModularRunner adaptation
4. **Config integration** — `HarvesterSection`, `ToModernConfig`/`FromModernConfig`
5. **TUI** — Dynamic form fields for vagrant/harvester
6. **Registration** — Wire up in `main.go` init()
7. **Testing** — Unit tests for config, validation, template rendering; integration test on real libvirt host

---

## Prerequisites for Users

- Linux host with KVM support (`/dev/kvm`)
- `vagrant` installed (>= 2.3)
- `vagrant-libvirt` plugin installed
- `libvirtd` running
- `ansible` installed (>= 2.12)
- Sufficient resources: ~50GB RAM and 24+ CPU cores for a 3-node cluster

**Yes.** `VagrantProvider.Validate()` checks all prerequisites. We also need to add a `Validate()` call at the top of `RunWithBuildDir()` in `runner_new.go` (currently missing). If validation fails, the TUI shows a blocking error before any workflow step runs.
---

## Open Questions

1. **Should we vendor the Ansible roles from ipxe-examples or call them externally?** Vendoring gives us control over versions and avoids external dependencies. Calling externally means users need to clone ipxe-examples. **Recommendation: vendor (embed) the critical roles and templates into saddle.**

2. **Airgap support from day one?** The ipxe-examples repo has a separate airgap variant with Docker registry, network isolation, etc. This is significantly more complex. **Recommendation: start with online mode only; add airgap as a follow-up feature.**
-> No Airgap support from day one

3. **Should Harvester be restricted to vagrant provider only?** Technically Harvester can be installed on bare metal or cloud instances via iPXE too. **Recommendation: allow any provider, but only implement+test vagrant for now. The orchestrator should be provider-agnostic where possible.**
-> We will create the minimum here (1-3 nodes running locally)

4. **How to handle the long boot time?** Harvester nodes take 15-30 minutes to install from PXE boot. The TUI needs to show progress. **Recommendation: poll the Harvester VIP endpoint and show status in the log viewer, same as existing deploy flow.**
**Progress bar: yes.** The VIP polling loop in `CustomApply` emits elapsed-time events via `logWriter`; the TUI log viewer already handles this. We can add an indeterminate spinner + "Xm elapsed / ~45m estimated" line during the wait phase.

**Daemon/TUI split: defer.** It's doable (separate binary + unix socket IPC + TUI as thin client), but it's a significant architectural change — bigger in scope than this feature. The current background-goroutine model is sufficient for now. Recommend tracking as a separate feature after Harvester works end-to-end.

5. **MAC address generation**: ipxe-examples uses hardcoded MACs. Should we auto-generate them? **Recommendation: auto-generate deterministic MACs from cluster name + node index to avoid conflicts between clusters.**
**Decided:** MACs are auto-generated (SHA-256 of cluster name + node index, formatted as `52:54:xx:xx:xx:xx`). Form exposes: hostname prefix, password, cluster token, memory (default 16384 MB), CPU (default 8), storage (default 250 GB).

---

## Minimal Deliverable Prompts

Each prompt is self-contained. Pass them sequentially; each one builds on the previous.

---

### Prompt 1 — Core: `ProviderWithCustomApply` interface + runner wiring

```
In internal/core/interfaces.go, add:

  type ProviderWithCustomApply interface {
      Provider
      CustomApply(ctx context.Context, config map[string]interface{}, buildDir string, logWriter io.Writer) (*InfrastructureOutputs, error)
  }

In internal/workflow/runner_new.go, make two changes:
1. At the top of RunWithBuildDir(), call r.Provider.Validate(providerConfig) and return the error immediately if it fails.
2. After Step 1 (GenerateInfrastructure), check:
     if p, ok := r.Provider.(core.ProviderWithCustomApply); ok {
         outputs, err := p.CustomApply(ctx, providerConfig, buildDir, os.Stdout)
         // skip steps 2-8, jump straight to displaySuccess
     }
   When the provider does NOT implement CustomApply, continue the existing steps 2-8 unchanged.

Include tests for both paths (custom apply taken / standard path taken) using a mock provider.
```

---

### Prompt 2 — Vagrant provider: config + prerequisite validator

```
Create internal/providers/vagrant/config.go and internal/providers/vagrant/validator.go.

VagrantConfig struct fields (all with defaults):
  NodeCount      int    // default 3
  NodeCPU        int    // default 8
  NodeMemoryMB   int    // default 16384
  NodeDiskGB     int    // default 250
  NetworkSubnet  string // default "192.168.0.0/24"
  PXEServerIP    string // default "192.168.0.254"
  NodePrefix     string // default "harvester-node"
  ClusterName    string // required

config.go: VagrantConfig struct + FromMap(map[string]interface{}) (*VagrantConfig, error) with defaults applied.

validator.go: CheckPrerequisites() error that verifies:
  - `vagrant` binary in PATH (exec.LookPath)
  - vagrant-libvirt plugin installed (run: vagrant plugin list, grep for "vagrant-libvirt")
  - libvirtd running (run: systemctl is-active libvirtd)
  - /dev/kvm exists (os.Stat)
Return a descriptive error for each missing prerequisite (not just the first one — collect all failures).

Include table-driven tests with testify/assert. For exec-based checks, test by verifying the error message format when the binary is missing (use a fake PATH via t.Setenv).
```

---

### Prompt 3 — Vagrant provider: Vagrantfile template + provider.go

```
Create internal/providers/vagrant/templates/Vagrantfile.tmpl and internal/providers/vagrant/provider.go.

Vagrantfile.tmpl generates:
  1. A libvirt network "harvester-{{ .ClusterName }}" with DHCP disabled, subnet from config
  2. PXE server VM (debian/bullseye64): static IP {{ .PXEServerIP }}, 2 CPU, 2048 MB RAM
     Synced folder: ./harvester-isos/{{ .HarvesterVersion }} → /harvester-media (type: :virtiofs or :nfs)
  3. N Harvester node VMs (no box, boot from network): {{ .NodeCPU }} CPU, {{ .NodeMemoryMB }} MB RAM,
     {{ .NodeDiskGB }} GB disk, PXE boot enabled, deterministic MAC per node

MAC generation helper in provider.go:
  GenerateMAC(clusterName string, nodeIndex int) string
  Use sha256(clusterName + strconv.Itoa(nodeIndex)), take bytes [0:3], format as "52:54:%02x:%02x:%02x:%02x" (first 4 bytes, KVM-safe prefix).

VagrantProvider implements core.Provider:
  Name()                    → "vagrant"
  Validate()                → calls CheckPrerequisites() from validator.go
  GenerateInfrastructure()  → renders Vagrantfile.tmpl → outputDir/Vagrantfile
  GetOutputs()              → runs `vagrant ssh-config pxe_server` in buildDir, parses HostName for PXE server IP;
                              returns InfrastructureOutputs{InstanceIPs: [pxeIP, node0IP, node1IP, ...]}
  GetRequiredFields()       → FormFields: node_count, node_cpu, node_memory_mb, node_disk_gb, network_subnet, pxe_server_ip, node_prefix
  GetDefaultConfig()        → map with the defaults above

CustomApply (implements ProviderWithCustomApply) — stub only in this prompt, full implementation in Prompt 5:
  return nil, fmt.Errorf("CustomApply: not yet implemented")

Include tests for Vagrantfile rendering (golden-file or substring checks) and GenerateMAC (same input → same output, different inputs → different outputs).
```

---

### Prompt 4 — Harvester orchestrator: templates

```
Create these templates in internal/orchestrators/harvester/templates/:

1. settings.yml.tmpl — ipxe-examples-compatible settings:
   harvester_iso_url, harvester_kernel_url, harvester_ramdisk_url, harvester_rootfs_url,
   harvester_cluster_token, harvester_password, harvester_mgmt_interface (default "eth0"),
   vip, vip_hw_address (empty for DHCP), ntp_servers list,
   per-node entries: name, mac, ip, role (create/join)

2. config-create.yaml.tmpl — Harvester install config for node-0 (create mode):
   scheme version, token, os.password, install.mode=create, install.managementInterface,
   install.vip, install.vipMode, system_settings.ntp-servers

3. config-join.yaml.tmpl — Harvester install config for join nodes:
   same structure but install.mode=join, install.serverUrl=https://{{ .VIP }}:443

4. ipxe-create.tmpl — iPXE script for node-0:
   #!ipxe, set base-url http://{{ .PXEServerIP }}/harvester-media,
   kernel ${base-url}/harvester-{{ .Version }}-vmlinuz-amd64 ... console=tty0 ... harvester.install.config_url=...
   initrd ${base-url}/harvester-{{ .Version }}-initrd-amd64
   boot

5. ipxe-join.tmpl — same as create but with join config URL

6. dhcpd.conf.tmpl — ISC-DHCP config:
   subnet declaration, one static host entry per Harvester node (MAC → IP), next-server = PXE server IP,
   filename "harvester-{{ .NodeName }}.ipxe"

7. setup-pxe.yml.tmpl — Ansible playbook (hosts: pxe_server):
   tasks: apt install isc-dhcp-server tftpd-hpa nginx,
   copy dhcpd.conf, copy per-node iPXE scripts to /var/lib/tftpboot/,
   copy per-node config YAMLs to /var/www/html/,
   configure nginx to serve /harvester-media (synced folder mount),
   restart services

Use Go template syntax ({{ }}) throughout. Keep templates focused; no logic beyond simple range/if.
```

---

### Prompt 5 — Harvester orchestrator: config + orchestrator.go + full CustomApply

```
Create internal/orchestrators/harvester/config.go and internal/orchestrators/harvester/orchestrator.go.

HarvesterConfig fields (with defaults):
  Version       string   // default "v1.4.1"
  ClusterToken  string   // default "token"
  Password      string   // default "p@ssword"
  VIP           string   // default "192.168.0.131"
  VIPMode       string   // default "DHCP" (options: DHCP, static)
  NTPServers    []string // default ["0.suse.pool.ntp.org"]
  DeployRancher bool     // default false

config.go: HarvesterConfig struct + FromMap().

HarvesterOrchestrator implements core.Orchestrator:
  Name()               → "harvester"
  Validate()           → version non-empty, VIP non-empty, node_count >= 1
  GeneratePlaybook()   → renders setup-pxe.yml.tmpl → outputDir/setup-pxe.yml
  GenerateInventory()  → renders hosts.ini with [pxe_server] group (PXE server IP from InfrastructureOutputs.InstanceIPs[0])
  GetRequiredFields()  → FormFields for all HarvesterConfig fields
  GetDefaultConfig()   → map with the defaults above
  GetModules()         → return empty []core.Module (no modules for Harvester)

Now implement the full VagrantProvider.CustomApply (replacing the stub from Prompt 3):
  1. Call EnsureISO(ctx, config["harvester_version"], buildDir, logWriter) — see Prompt 6
  2. Run `vagrant up pxe_server` in buildDir, stream stdout/stderr to logWriter
  3. Run `ansible-playbook -i hosts.ini setup-pxe.yml` in buildDir, stream to logWriter
  4. Run `vagrant up harvester-node-0 harvester-node-1 ...` (one per node_count), stream to logWriter
  5. Poll https://<VIP>:443 every 30s up to 60 attempts (30 min), writing
     "Waiting for Harvester cluster... Xm elapsed" to logWriter each attempt.
     Use http.Client with TLS InsecureSkipVerify and 10s timeout.
     On HTTP 200/302/401 (any response), consider cluster ready.
  6. Return InfrastructureOutputs{InstanceIPs: nodeIPs, Metadata: {"vip": vip}}

Include tests for HarvesterConfig.FromMap(), Validate(), and template rendering.
```

---

### Prompt 6 — ISO caching

```
Create internal/providers/vagrant/iso_cache.go.

func CacheDir(projectRoot, version string) string
  → filepath.Join(projectRoot, "harvester-isos", version)

func EnsureISO(ctx context.Context, projectRoot, version string, logWriter io.Writer) error
  Artifacts to ensure (all four):
    harvester-<version>-amd64.iso
    harvester-<version>-vmlinuz-amd64
    harvester-<version>-initrd-amd64
    harvester-<version>-rootfs-amd64.squashfs
  For each artifact:
    - Check if filepath.Join(CacheDir(projectRoot, version), artifact) exists → skip if yes
    - Otherwise download from https://releases.rancher.com/harvester/<version>/<artifact>
    - Write progress to logWriter: "Downloading <artifact>... X MB / Y MB"
    - Use http.Get with context, write to file with os.Create
    - On error, delete the partial file
  Create the cache dir with os.MkdirAll before any downloads.

Add `harvester-isos/` to the project's .gitignore.

Include tests:
  - CacheDir() returns correct path
  - EnsureISO skips download when files already exist (create dummy files in t.TempDir())
  - EnsureISO returns error on failed download (use httptest.NewServer returning 500)
```

---

### Prompt 7 — Config integration: HarvesterSection

```
Update internal/config/clusters.go to support Harvester clusters.

Add HarvesterSection struct:
  ClusterToken  string
  Password      string
  VIP           string
  VIPMode       string
  NTPServers    []string
  DeployRancher bool

Add field HarvesterConfig *HarvesterSection to ClusterConfig (alongside the existing ProviderConfig / OrchestratorConfig fields).

Update ToModernConfig() and FromModernConfig() (or the YAML marshal/unmarshal logic) to include HarvesterConfig when Provider.Type == "vagrant" and Kubernetes.Distribution == "harvester".

Include table-driven tests for round-trip serialization of a Harvester cluster config (marshal → unmarshal → assert equal).
```

---

### Prompt 8 — TUI: dynamic form fields for vagrant + harvester

```
Update internal/tui/views/createform.go to add vagrant and harvester support.

Changes:
1. Add "vagrant" to the provider select field options list.
2. Add "harvester" to the Kubernetes Distribution select field options list.
3. Make field visibility dynamic based on current selections:
   - provider == "aws":     show AWS fields (credentials, region, subnet_id, sg_id, os_image, instance_type)
                            hide Vagrant fields (node_cpu, node_memory_mb, node_disk_gb, network_subnet, pxe_server_ip)
   - provider == "vagrant": hide AWS fields, show Vagrant fields
   - distribution == "rke2" or "k3s": show RKE2/K3s fields (k8s_version, rancher_version, bootstrap_password, image_tag, debug)
                                      hide Harvester fields
   - distribution == "harvester":     hide RKE2/K3s fields, show Harvester fields
                                      (harvester_version, cluster_token, password, vip, vip_mode, ntp_servers)

Follow the same visibility pattern already used in the form for other conditional fields.

Defaults for Vagrant fields: node_cpu="8", node_memory_mb="16384", node_disk_gb="250".
Default for harvester_version: "v1.4.1".

Include tests for field visibility: given provider/distribution selection, assert which fields are visible/hidden.
```

---

### Prompt 9 — Registration + wiring

```
Wire up the new provider and orchestrator.

1. In main.go (or wherever providers/orchestrators are registered via init()):
   - Import internal/providers/vagrant
   - Import internal/orchestrators/harvester
   - Register: core.DefaultRegistry.RegisterProvider(&vagrant.VagrantProvider{})
   - Register: core.DefaultRegistry.RegisterOrchestrator(&harvester.HarvesterOrchestrator{})

2. Check for any switch statements or string slices in the TUI or workflow that enumerate
   valid providers (e.g., []string{"aws"}) or orchestrators — add "vagrant" and "harvester" there.

3. Run `make build` and fix any compile errors.

4. Run `make test` and fix any test failures introduced by the new imports.

No new logic — just wiring. Keep the diff minimal.
```

---

### Prompt 10 — Progress indicator for Harvester VIP wait

```
Improve the Harvester VIP polling UX in the TUI log viewer.

In VagrantProvider.CustomApply Phase 5 (VIP polling loop), replace plain log lines with structured progress events:
  - Each poll attempt writes to logWriter:
      fmt.Fprintf(logWriter, "[harvester-wait] attempt=%d elapsed=%s\n", attempt, elapsed)
  - On success: fmt.Fprintf(logWriter, "[harvester-ready] vip=%s\n", vip)
  - On timeout: return a descriptive error including elapsed time

In internal/tui/views/ (the deploy log view), detect lines starting with "[harvester-wait]" and render them as an updating status line (overwrite previous status line) showing:
  "Waiting for Harvester cluster...  12m30s elapsed (est. ~30m)"
Replace the status line in-place rather than appending, so the log doesn't fill with 60 identical lines.
On "[harvester-ready]", replace the status line with "Harvester cluster ready at https://<vip>".

Keep all other log lines unchanged. No changes to the polling interval or retry logic.
```

---

## Implementation Status (2026-10-20)

Core generation + deploy backend implemented and unit-tested.

**Done:**
- Prompt 1 — `core.ProviderWithCustomApply` (+ `PlannedOutputs`); runner validates the
  provider and delegates to `runCustomApply` (generates playbook+inventory, then CustomApply).
- Prompt 2 — `vagrant` config + `validator.go` (`CheckPrerequisites`, collects all failures).
- Prompt 3 — `Vagrantfile.tmpl` + `provider.go` (`GenerateMAC`, `ComputeTopology`,
  `GenerateInfrastructure`, `PlannedOutputs`, `GetOutputs`).
- Prompt 4 — Harvester templates: `settings.yml`, `config.yaml` (create/join in one,
  role-driven), `ipxe.tmpl`, `dhcpd.conf`, `setup-pxe.yml` (per-node tasks via Go `range`,
  no Jinja to avoid delimiter clash).
- Prompt 5 — Harvester config + orchestrator (`GeneratePlaybook`/`GenerateInventory`);
  full `CustomApply` (ensure media → up pxe → ansible → up nodes → poll VIP).
- Prompt 6 — `iso_cache.go` (`CacheDir`, `EnsureISO` with progress; `harvester-isos/` gitignored).
- Prompt 7 — Round-trips through the existing map-based `ProviderConfig`/`OrchestratorConfig`
  (`ToModernConfig`); no dedicated `HarvesterSection` struct was needed.
- Prompt 8 — TUI: Harvester selectable; Vagrant↔Harvester coupling; AWS fields hidden for
  the local-VM path; dedicated `handleLocalVMSubmit`/`createLocalVMCluster` (no cloud creds).
- Prompt 9 — Provider/orchestrator already registered in `main.go`.

**Design choices / deviations from the prompts:**
- Node topology (names/MACs/IPs/roles) is shared via `core.Topology` written to
  `topology.json` in the build dir (provider writes, orchestrator reads) — keeps the two
  packages decoupled instead of importing one from the other.
- `Provider.Validate` checks config only; host prerequisites are checked in `CustomApply`
  (fail-fast at deploy) so merely validating a config doesn't require a libvirt host.
- `config.yaml`/`ipxe` templates are consolidated (role-driven) rather than separate
  create/join files.

**Deferred:**
- Prompt 10 — the in-place TUI progress line. `CustomApply` already emits structured
  `[harvester-wait]`/`[harvester-ready]` events; the log view renders them as plain lines.
- Per-node sizing/VIP/token in the create form (CPU/mem/disk/vip/token) currently use
  provider/orchestrator defaults; advanced form fields are a follow-up.
- End-to-end deploy is unverified (requires a libvirt/KVM host); only generation and unit
  tests are exercised here.

---

## Architecture change (2026-10-20): PXE → ISO-boot, libvirt-direct

The iPXE-netboot approach was replaced per user requirement ("no iPXE; boot the
ISO, auto-install, auto-join"). Also dropped Vagrant: its box/SSH model doesn't
fit installing an appliance OS from an ISO (SSH-convergence hang + reinstall
loop). The provider id stays "vagrant" for config compatibility but now drives
libvirt directly.

**New flow:**
- Each node is a libvirt domain (virsh + qemu-img): blank qcow2 disk + the
  Harvester ISO as a CD-ROM, boot order `hd,cdrom` (blank disk falls through to
  the installer, then boots the installed disk — no reinstall loop).
- The ISO is repacked once (xorriso) to patch GRUB for an unattended boot:
  `harvester.install.automatic=true` + `harvester.install.config_url=http://<gw>:8181/config`.
- A built-in HTTP config server (not PXE/DHCP) serves each node its install
  config by source IP: node 0 gets `mode: create`, the rest `mode: join` with
  `server_url`+`token`. Create node boots first; we wait for the VIP, then boot
  the joiners.
- Only the ISO is needed (no separate vmlinuz/initrd/rootfs downloads).

**Files:** `network.xml.tmpl`, `domain.xml.tmpl` (new); `config.yaml.tmpl` kept
(install config); PXE templates (ipxe/dhcpd/setup-pxe/settings) removed.
`iso_cache.go` adds `RepackAutoInstallISO`/`AutoInstallISOPath`; `config_server.go`
is new; validator checks virsh/qemu-img/xorriso; `core.Topology` now carries the
gateway instead of a PXE server IP.

**Needs real-host validation (can't test here):** GRUB patch for the specific
Harvester ISO version; boot-order fallthrough behavior; q35/virtio NIC naming
(`management_interface.name`, defaulted to `enp1s0`); libvirt file permissions
for qemu to read the ISO/disks. Cluster teardown (virsh destroy/undefine + net +
disks) is not yet wired into the delete flow.
