# Architecture

## Overview

rancher-tui follows a **Provider/Orchestrator** pattern. Cloud providers (AWS) handle infrastructure provisioning via OpenTofu templates. Orchestrators (RKE2, K3s) handle Kubernetes deployment via Ansible playbooks. A thread-safe registry connects them at runtime.

```
+-------------------------------------+
|         User Interface              |
|    (Fullscreen TUI / CLI)           |
+-----------------+-------------------+
                  |
                  v
+-------------------------------------+
|      Configuration Layer            |
|  config.yaml      (clusters)        |
|  cloud-credentials.yaml (AWS creds) |
|  profiles.yaml    (infra presets)   |
|  amis.yaml        (AMI catalog)     |
+-----------------+-------------------+
                  |
                  v
+-------------------------------------+
|         Core Registry               |
|  Providers (AWS)                    |
|  Orchestrators (RKE2, K3s)         |
+-----------------+-------------------+
                  |
            +-----+-----+
            v           v
      +---------+  +-----------+
      |Provider |  |Orchestr.  |
      | (AWS)   |  |(RKE2/K3s) |
      +----+----+  +-----+-----+
           |              |
           v              v
      +---------+  +-----------+
      |OpenTofu |  |  Ansible  |
      |Templates|  | Playbooks |
      +---------+  +-----------+
```

## Package Structure

```
internal/
├── cluster/          # CLI cluster commands (CREATE, LIST, DELETE)
├── config/           # YAML persistence
│   ├── clusters.go   # ClustersConfig, ClusterConfig, RancherSection
│   ├── config.go     # Modern Config format for workflows
│   ├── amis.go       # AMI catalog (distro/region/AMI-ID mappings)
│   ├── profiles.go   # Infrastructure profiles
│   ├── bridge.go     # Legacy config conversion
│   └── validation.go # Path expansion, config validation
├── core/             # Core abstractions
│   ├── interfaces.go # Provider, Orchestrator, Generator interfaces
│   ├── registry.go   # Thread-safe component registry
│   └── types.go      # ProviderType, OrchestratorType, FormField, InfrastructureOutputs
├── credentials/      # AWS credential store (static keys + SSO profile refs)
├── generator/        # Go text/template renderer
│   └── renderer.go   # Render, RenderString, RenderWithFuncs
├── orchestrators/
│   ├── rke2/         # RKE2 orchestrator + Ansible templates
│   ├── k3s/          # K3s orchestrator + Ansible templates
│   └── harvester/    # Harvester orchestrator (scaffold: config/validation only)
├── providers/
│   ├── aws/          # AWS provider + Terraform templates
│   │   ├── config.go # AWSConfig with FromMap (handles int/float64)
│   │   └── provider.go # GenerateInfrastructure, GetOutputs
│   └── vagrant/      # Vagrant/libvirt local provider (scaffold)
├── tui/              # Terminal UI (Bubbletea)
│   ├── root.go       # State machine, layout manager
│   ├── header.go     # Header bar
│   ├── footer.go     # Footer with live log panel
│   ├── keymap/       # nvim-style key-sequence resolver (pure logic) + which-key
│   └── views/        # View components
│       ├── messages.go      # AppState enum, message types
│       ├── dashboard.go     # Astronvim-style home screen
│       ├── clusterlist.go   # Cluster table with auto-refresh
│       ├── createform.go    # 19-field cluster creation form
│       ├── deletemodal.go   # Delete confirmation + tofu destroy
│       ├── upgradeform.go   # Rancher upgrade form
│       ├── color_helper.go  # Status colors, formatting
│       ├── credentialsform.go / credentialslist.go
│       ├── profilesform.go / profileslist.go
│       └── amisform.go / amislist.go
├── upgrade/          # Rancher upgrade runner
│   ├── runner.go     # UpgradeConfig, Runner, templateData()
│   └── templates/    # Ansible upgrade playbook + inventory
├── utils/            # Zap logger initialization
└── workflow/         # Deployment orchestration
    └── runner_new.go # ModularRunner (9-step deploy pipeline)
```

## Interface Contracts

### Provider (internal/core/interfaces.go)

```go
type Provider interface {
    Name() ProviderType
    Validate(config map[string]interface{}) error
    GenerateInfrastructure(ctx context.Context, config map[string]interface{}, outputDir string) error
    GetOutputs(ctx context.Context, buildDir string) (*InfrastructureOutputs, error)
    GetRequiredFields() []FormField
    GetDefaultConfig() map[string]interface{}
}
```

### Authentication hooks (optional interfaces)

Providers may additionally implement:

```go
type SessionChecker interface {
    CheckSession(ctx context.Context, config map[string]interface{}) error
}

type AuthEnvProvider interface {
    AuthEnv(config map[string]interface{}) []string
}
```

`SessionChecker` runs as a preflight before any infrastructure work (used by AWS to
fail fast on an expired SSO session). `AuthEnvProvider` supplies the environment for
`tofu` — credentials are never rendered into generated HCL.

### Orchestrator (internal/core/interfaces.go)

```go
type Orchestrator interface {
    Name() OrchestratorType
    Validate(config map[string]interface{}) error
    GeneratePlaybook(ctx context.Context, config map[string]interface{}, outputDir string) error
    GenerateInventory(ctx context.Context, outputs *InfrastructureOutputs, config map[string]interface{}, outputDir string) error
    GetRequiredFields() []FormField
    GetDefaultConfig() map[string]interface{}
    GetModules() []Module
}
```

## Authentication

Two AWS authentication modes are supported, selected per credential and recorded
per cluster in `provider.config`:

| Mode | Stored where | `tofu` receives |
|---|---|---|
| `static` | keys in `cloud-credentials.yaml` / `config.yaml` | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` |
| `sso` | only the **profile name** (the token cache lives in `~/.aws/sso/cache`) | `AWS_PROFILE` |

Generated HCL never contains credentials: `provider "aws"` gets `profile = "..."` in
SSO mode, and the environment is built by `internal/providers/aws/auth.go::EnvFor`,
which also scrubs inherited `AWS_*` static variables (they would otherwise take
precedence over `profile`).

Flow:

1. `credentialsform` writes a credential of type `static` or `sso`.
2. `credentialslist` shows SSO session status and the cached token lifetime; `r`
   refreshes both. Login happens outside the TUI (`aws sso login`).
3. The create form stores `auth` + `profile` (SSO) or keys (static) in `config.yaml`.
4. `ModularRunner` runs the `SessionChecker` preflight, then `tofu init/apply` with
   `AuthEnv`.
5. `tofu destroy` (CLI and TUI) applies the same preflight and environment.

## Data Flows

### Deployment Flow

```
TUI Create Form (19 fields)
    → config.yaml saved (ClusterConfig)
    → ToModernConfig() → Config struct
    → Provider.GenerateInfrastructure() → clusters/<name>/main.tf
    → tofu init && tofu apply → EC2 instances
    → Provider.GetOutputs() → IPs, DNS names
    → Orchestrator.GenerateInventory() → clusters/<name>/hosts.ini
        [init] = first node, [join] = remaining nodes
    → Orchestrator.GeneratePlaybook() → clusters/<name>/site.yml
        Play 1: Init node (install RKE2/K3s)
        Play 2: Join nodes
        Play 3: Addons (cert-manager + Rancher if enabled)
    → ansible-playbook site.yml -i hosts.ini
    → Status → "running", Rancher URL saved
```

### Delete Flow

```
TUI: press 'x' → DeleteModalModel confirms
    → Status → "deleting" (saved immediately)
    → Background goroutine: tofu destroy -auto-approve
    → os.RemoveAll(clusters/<name>/)
    → cfg.DeleteCluster(name) → config.yaml updated
    → TUI auto-refreshes (cluster disappears)
```

### Upgrade Flow

```
TUI: press 'u' (rancher.deploy must be true)
    → StateUpgradeForm (pre-filled from cluster config)
    → Background goroutine: upgrade.Runner.Run()
        → Render inventory (single init node)
        → Render upgrade playbook (helm upgrade)
        → ansible-playbook -i hosts.ini upgrade.yml
    → Update config.yaml with new version
    → Back to StateClusterList
```

## TUI State Machine

States defined in `internal/tui/views/messages.go`:

| State | View | Entry |
|---|---|---|
| `StateDashboard` | Astronvim-style home screen (banner + action menu) | Default / `esc` from any view |
| `StateClusterList` | Cluster table + optional log panel | From dashboard / after operations |
| `StateCreateForm` | Cluster creation form | `n` / `c` key |
| `StateDeleteConfirm` | Confirmation modal | `x` key |
| `StateCredentialsList` | Credentials table | `Ctrl+X` |
| `StateCredentialsForm` | Credentials editor | From credentials list |
| `StateProfilesList` | Profiles table | `Ctrl+P` |
| `StateProfilesForm` | Profile editor | From profiles list |
| `StateAMIsList` | AMI catalog browser | `Ctrl+A` |
| `StateAMIsForm` | AMI editor | From AMI list |
| `StateUpgradeForm` | Rancher upgrade form | `u` key |
| `StateHelp` | Help overlay | `?` key |

`StateDashboard` is the initial state. Key input is also fed through the nvim-style
resolver in `internal/tui/keymap/` (leader = space, `j/k`, `gg/G`, `:` command, `/` search,
which-key popup), which maps key sequences to stable `CommandID`s decoupled from bubbletea.

Layout: Header (2 lines) + Content (remaining) + Footer (3 lines). Log panel takes 33% when active.

## Configuration Schemas

### ClusterConfig (config.yaml)

```yaml
clusters:
  <name>:
    provider:
      type: aws
      config:
        # SSO clusters:
        auth: sso
        profile: acme-prod
        # Static clusters (legacy and explicit):
        # auth: static
        # access_key: AKIA...
        # secret_key: ...
        region: us-east-1
        instance_type: t3.xlarge
        subnet_id: subnet-...
        security_group_id: sg-...
        ami: ami-...
    kubernetes:
      distribution: rke2  # or k3s
      config: { version }
    rancher:
      version: "2.11.7"
      deploy: true
      prime: false
      bootstrap_password: admin
      audit_log: false
      audit_log_level: 0
    ssh: { key_name, private_key_path, user }
    cluster: { node_prefix, instance_count }
    status: running  # creating, running, deleting, failed, upgrading
    instance_ips: [...]
    instance_dns: [...]
    rancher_url: https://...
```

### Template Rendering

`internal/generator/renderer.go` provides:
- `Render(ctx, templatePath, data, outputPath)` — file template to file
- `RenderString(ctx, name, templateStr, data, outputPath)` — string template to file
- `RenderWithFuncs(ctx, templatePath, data, outputPath, funcMap)` — with custom functions

Orchestrators use module-based composition: init.yml.tmpl, join.yml.tmpl, addons.yml.tmpl are rendered individually and composed into playbook.yml.tmpl.
