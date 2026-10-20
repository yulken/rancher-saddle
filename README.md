# rancher-tui

Automated deployment tool for Kubernetes clusters (RKE2/K3s) with Rancher on AWS EC2 using OpenTofu and Ansible, featuring an interactive fullscreen TUI.

## Features

- **Astronvim-style Dashboard** - ASCII-banner home screen with a centered action menu and recent-cluster summary
- **nvim-style Keybindings** - Leader key (space), `j/k` navigation, `gg/G`, `:` command / `/` search modes, which-key popup
- **Interactive Fullscreen TUI** - Bubbletea-based terminal interface with live log panel
- **AWS EC2 Deployment** - Automated multi-node instance provisioning with OpenTofu
- **RKE2 & K3s Support** - Choose between production-grade RKE2 or lightweight K3s
- **Harvester & Vagrant (scaffolded)** - Harvester HCI orchestrator and a Vagrant/libvirt local provider are wired in; full install flows land in a later phase
- **Rancher Management** - Deploy Rancher (standard or Prime) with configurable bootstrap password
- **Multi-Cluster Management** - Create, monitor, and delete clusters independently
- **Live Deployment Logs** - Real-time log viewer occupying 33% of the screen
- **Credentials & Profiles** - Save and reuse AWS credentials and infrastructure profiles
- **Infrastructure Cleanup** - Delete clusters with full `tofu destroy` and config removal

## Prerequisites

### Required Tools
- [OpenTofu](https://opentofu.org/) (or Terraform) - `tofu` command in PATH
- [Ansible](https://www.ansible.com/) - `ansible-playbook` command in PATH
- Go 1.24.9+
- AWS Account with appropriate permissions

### AWS Requirements
- Valid AWS access key and secret key
- Existing VPC with subnet
- Security group with required ports:
  - SSH (22)
  - HTTP/HTTPS (80, 443)
  - Kubernetes API (6443)
  - RKE2 supervisor (9345) / K3s (10250)
- EC2 key pair for SSH access

## Installation

```bash
git clone https://github.com/Felipalds/rancher-tui.git
cd rancher-tui
go build -o rancher-tui
```

## Usage

### Fullscreen TUI (Recommended)

```bash
./rancher-tui
```

The TUI provides a fullscreen experience with:

- **Dashboard** - The home screen: ASCII banner, action menu (Clusters, Create, Credentials, Profiles, AMIs, Quit) and a recent-cluster summary. `esc` from any view returns here.
- **Cluster List** - Table showing all clusters with name, status, nodes, provider, region, Rancher URL, and age
- **Create Form** - Full cluster configuration form
- **Live Logs** - Press Enter on a cluster to show real-time deployment logs (33% of screen)
- **Delete** - Press `x` to destroy infrastructure and remove cluster
- **Upgrade** - Press `u` to upgrade Rancher on a running cluster
- **Auto-refresh** - Cluster status and logs update every second

#### nvim-style Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` (or `↓`/`↑`) | Move down / up |
| `gg` / `G` | Jump to top / bottom |
| `enter` | Select / open |
| `esc` | Back to dashboard |
| `<space>` (leader) | Open which-key popup |
| `:` | Command mode |
| `/` | Search |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

#### Cluster-list Keybindings

| Key | Action |
|-----|--------|
| `n` / `c` | Create new cluster |
| `x` | Delete selected cluster |
| `u` | Upgrade Rancher (if deployed) |
| `r` | Manual refresh |
| `Enter` | Toggle log viewer |
| `Ctrl+X` | Manage credentials |
| `Ctrl+P` | Manage profiles |
| `Ctrl+A` | AMI catalog |

#### Create Form Fields

| Field | Default | Description |
|-------|---------|-------------|
| Provider | AWS | Cloud provider |
| Credentials | - | Saved AWS credential set |
| K8s Distribution | RKE2 | RKE2 or K3s |
| Cluster Name | my-cluster | Unique cluster identifier |
| Node Prefix | k8s-node | EC2 instance name prefix |
| Region | us-east-1 | AWS region |
| Subnet ID | - | VPC subnet |
| Security Group ID | - | Security group |
| AMI ID | - | Ubuntu 22.04 recommended |
| Instance Type | t3.xlarge | EC2 instance type |
| Instance Count | 3 | Number of nodes (HA) |
| SSH Key Name | - | AWS key pair name |
| SSH Private Key Path | - | Path to .pem file |
| SSH User | ubuntu | SSH username |
| K8s Version | v1.33.7+rke2r1 | Distribution version |
| Deploy Rancher | No | Enable Rancher deployment |
| Rancher Prime | No | Use Rancher Prime (SUSE registry) |
| Rancher Version | 2.11.7 | Rancher chart version |
| Bootstrap Password | admin | Rancher initial admin password |

### CLI Commands

```bash
# Create cluster (opens TUI form)
./rancher-tui create my-cluster

# List all clusters
./rancher-tui list

# Delete cluster (with confirmation)
./rancher-tui delete my-cluster

# Delete without confirmation
./rancher-tui delete my-cluster --force

# List available providers
./rancher-tui list-providers

# List available orchestrators
./rancher-tui list-orchestrators
```

## How It Works

### Deployment Flow

1. User configures cluster via TUI form
2. Configuration saved to `config.yaml`
3. OpenTofu provisions EC2 instances (`clusters/<name>/main.tf`)
4. SSH readiness check on all instances
5. Ansible inventory generated (`clusters/<name>/hosts.ini`)
   - First node in `[init]` group (control plane + Rancher host)
   - Remaining nodes in `[join]` group (HA members)
6. Ansible playbook generated and executed (`clusters/<name>/site.yml`)
   - Installs RKE2/K3s on init node
   - Joins remaining nodes to cluster
   - Deploys cert-manager (v1.17.2) and Rancher (if enabled)
7. Cluster status updates to `running`, Rancher URL shown in table

### Delete Flow

1. User presses `x` and confirms
2. Status set to `deleting` (visible in TUI immediately)
3. Background goroutine runs `tofu destroy -auto-approve`
4. Build directory removed (`clusters/<name>/`)
5. Cluster entry removed from `config.yaml`
6. TUI auto-refreshes to reflect changes

### Rancher Prime vs Standard

When **Rancher Prime** is enabled:
- Helm repo: `https://charts.rancher.com/server-charts/prime`
- Container image: `registry.suse.com/rancher/rancher`
- System default registry: `registry.suse.com`

When **standard Rancher** (default):
- Helm repo: `https://releases.rancher.com/server-charts/latest`
- Default upstream container images

## Configuration Files

| File | Purpose |
|------|---------|
| `config.yaml` | Cluster configurations (auto-generated, contains secrets - 0600) |
| `cloud-credentials.yaml` | Saved AWS credentials (0600) |
| `profiles.yaml` | Saved infrastructure profiles (0600) |
| `amis.yaml` | AMI catalog - distro/region/AMI-ID mappings |
| `clusters/<name>/` | Per-cluster build directory (Terraform state, playbooks) |
| `logs/<name>.log` | Per-cluster deployment/deletion logs |

## Roadmap

rancher-tui is evolving from a RKE2/K3s + Rancher deployer into a TUI for the whole
Rancher product family. The work is phased:

- **Phase 1 (done):** rebrand to rancher-tui, Astronvim-style dashboard home screen,
  nvim-style keybinding/command engine, and Harvester + Vagrant/libvirt scaffolding.
- **Next:** full Harvester installs (cloud + local via Vagrant/libvirt), Longhorn,
  and an observability stack; production vs. test environment presets.

See `feats/rancher-tui-transformation.md` for the detailed plan and `feats/2-harvester-vagrant.md`
for the Harvester design.

## Documentation

- **[docs/architecture.md](docs/architecture.md)** - Technical architecture, package structure, data flows, interfaces
- **[docs/product.md](docs/product.md)** - Product decisions, feature list, version history, roadmap

## Testing

```bash
make test           # Run all tests
make test-cover     # Run tests with coverage report
make test-verbose   # Run tests with verbose output
make lint           # Run go vet
```

Tests use [testify](https://github.com/stretchr/testify) with table-driven patterns. See `internal/config/validation_test.go` for the established style.

## Contributing

1. Write a feature proposal in `feats/<name>.md`
2. Follow TDD: write tests first, then implement
3. Run `make test` before submitting
4. Rename completed features to `feats/x-(completed)-<name>.md`

## Troubleshooting

### OpenTofu Errors
- Verify AWS credentials are valid
- Check subnet and security group exist in the specified region
- Ensure IAM permissions include EC2 full access

### Ansible Errors
- Verify SSH key path is correct and has 0600 permissions
- Check security group allows SSH (port 22)
- Check logs: `logs/<cluster-name>.log`

### Only 1 Node Created
- Ensure Instance Count field is set (default: 3)
- This was a known bug (type mismatch in config passing) - fixed in v0.5

### Rancher Access
- Use the DNS name shown in the Rancher URL column
- Wait 5-10 minutes after deployment for Rancher to initialize
- Default login: admin / (your bootstrap password)

## Security Considerations

- `config.yaml` and `cloud-credentials.yaml` contain secrets - never commit to git
- All credential files use 0600 permissions
- SSH private keys should have 0600 permissions
- Use restrictive security groups in production

## Author

Felipalds @ SUSE (luiz.rosa@suse.com)

## Acknowledgments

- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [OpenTofu](https://opentofu.org/) - Infrastructure as Code
- [Ansible](https://www.ansible.com/) - Configuration management
- [Rancher](https://rancher.com/) - Kubernetes management
