# Product Documentation

## Vision

rancher-tui automates Kubernetes cluster deployment on cloud infrastructure. It targets engineers who need to quickly spin up RKE2/K3s clusters with Rancher for development, testing, or demos — without manually writing Terraform and Ansible.

## Scope

- **Cloud providers**: AWS EC2 (extensible to Azure, GCP, vSphere)
- **K8s distributions**: RKE2, K3s (extensible to kubeadm, minikube)
- **Management plane**: Rancher (standard and Prime)
- **Interface**: Fullscreen TUI (primary), CLI commands (secondary)

## Features

### Implemented

| Feature | Version | Description |
|---|---|---|
| Interactive TUI | v0.1 | Bubbletea-based fullscreen terminal UI |
| RKE2 deployment | v0.1 | Production-grade K8s via Ansible |
| AWS EC2 provisioning | v0.1 | OpenTofu-based infrastructure |
| Rancher installation | v0.1 | cert-manager + Rancher Helm chart |
| Multi-cluster management | v0.3 | Create, monitor, delete clusters independently |
| Cluster state tracking | v0.3 | Status persistence in config.yaml |
| SSH readiness checks | v0.3 | Retry loop before Ansible runs |
| K3s support | v0.4 | Lightweight K8s distribution |
| YAML configuration | v0.4 | Migrated from JSON |
| Fullscreen state machine | v0.4 | Proper TUI navigation with 13 states |
| Rancher Prime | v0.5 | SUSE registry, prime Helm chart |
| Multi-node HA | v0.5 | Fixed instance_count pipeline |
| Live log panel | v0.5 | 33% of screen, 1-second refresh |
| Real infrastructure delete | v0.5 | tofu destroy + cleanup |
| Credentials management | v0.5 | Save/load AWS credential sets |
| Profiles management | v0.5 | Save/load infrastructure presets |
| AMI catalog | v0.5+ | Managed AMI table with 36 defaults |
| Rancher upgrade | v0.5+ | In-place Rancher version upgrade via Ansible |

### UX Decisions

| Decision | Choice | Reason |
|---|---|---|
| TUI framework | Bubbletea | Best Go TUI framework, Elm architecture |
| Log panel size | 33% of terminal | Enough to read logs without losing cluster context |
| Auto-refresh | 1 second interval | Fast feedback without flickering |
| Keybinding style | Single keys + Ctrl combos | Vim-inspired, discoverable via footer |
| Delete confirmation | Modal with y/n | Prevents accidental infrastructure destruction |
| Background operations | Goroutines | Non-blocking TUI during long deploy/delete |
| Cluster sort | Alphabetical | Stable row order across refreshes |

### Keybindings (Cluster List)

| Key | Action |
|---|---|
| `n` / `c` | Create new cluster |
| `x` | Delete selected cluster |
| `u` | Upgrade Rancher (if deployed) |
| `Enter` | Toggle log viewer |
| `Ctrl+X` | Manage credentials |
| `Ctrl+P` | Manage profiles |
| `Ctrl+A` | AMI catalog |
| `?` | Help |
| `q` / `Ctrl+C` | Quit |

## Rancher Prime vs Standard

| Setting | Rancher Prime | Rancher (Latest) |
|---|---|---|
| Helm repo | `rancher-prime` from `charts.rancher.com` | `rancher-latest` from `releases.rancher.com` |
| Container image | `registry.suse.com/rancher/rancher` | Default upstream |
| System registry | `registry.suse.com` | Not set |
| Extra env vars | `CATTLE_DEBUG`, `RANCHER_VERSION_TYPE=prime`, `CATTLE_BASE_UI_BRAND=suse` | Not set |

## Version History

### v0.5.0 — rancher-tui rebrand (2026-10-20, Current)
- Renamed project/module/binary `rancher-saddle` → `rancher-tui`
- Astronvim/Tornado-style **dashboard** home screen (`StateDashboard`, now the initial state)
- **nvim-style keybinding/command engine** (`internal/tui/keymap`): leader = space, `j/k`,
  `gg/G`, `:` command mode, `/` search, which-key popup
- **Harvester orchestrator** and **Vagrant/libvirt provider** scaffolded (config, validation,
  form fields, registry wiring; install generation not implemented yet)
- Phase 1 of the broader transformation — see `feats/rancher-tui-transformation.md`

### v0.4.x (earlier)
- AMI catalog management (`amis.yaml` with 36 default entries)
- Rancher upgrade from TUI (`u` key)
- Keybinding remap: `x` = delete, `Ctrl+A` = AMIs
- Footer shows all keybindings per view

### v0.5 (2026-02-12)
- Rancher Prime support (SUSE registry, prime Helm chart)
- Deploy Rancher toggle, version, bootstrap password fields
- Fixed multi-node cluster creation (instance_count type mismatch)
- Real infrastructure delete (tofu destroy + cleanup)
- Live log panel (33% of screen, 1-second refresh)
- Rancher URL column in cluster list
- Auto-refresh every 1 second
- Fullscreen layout, dynamic column sizing
- Credentials and profiles management
- cert-manager bumped to v1.17.2

### v0.4 (2026-02-10)
- K3s orchestrator support
- TUI distribution selector
- JSON to YAML migration
- Fixed Jinja2 template escaping
- Path expansion for SSH keys
- Fullscreen TUI refactor with state machine

### v0.3 (2026-02-09)
- Multi-cluster management
- Cluster state tracking
- Isolated build directories per cluster
- SSH readiness checks
- Rancher DNS name support

### v0.1 (Initial Release)
- Interactive TUI configuration
- RKE2 cluster deployment
- AWS EC2 provisioning with OpenTofu
- Ansible playbook generation
- Rancher installation

## Roadmap

Phased transformation into a TUI for the whole Rancher product family
(see `feats/rancher-tui-transformation.md`):

- [x] **Phase 1:** rebrand to rancher-tui, dashboard home screen, nvim-style command engine,
      Harvester + Vagrant/libvirt scaffolding
- [ ] **Phase 2:** full Harvester installs — cloud and local via Vagrant/libvirt
      (see `feats/2-harvester-vagrant.md`)
- [ ] Longhorn distributed storage management
- [ ] Observability stack (Rancher monitoring / Prometheus, Grafana)
- [ ] Production vs. test environment presets
- [ ] Error status for resumable deploys (continue after partial failure)
- [ ] New cloud providers (Azure, GCP, vSphere)
- [ ] Cluster scaling (add/remove nodes post-deploy)
- [ ] Import existing clusters
