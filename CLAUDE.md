# CLAUDE.md — LLM Instructions for rancher-tui

## Project Overview

TUI for creating and managing the Rancher product family (Rancher, Harvester, Longhorn,
observability, RKE2, K3s) across cloud and local environments. Today it deploys RKE2/K3s
clusters with Rancher on AWS EC2; Harvester (cloud + local via Vagrant/libvirt) and other
products are being added incrementally — see `feats/rancher-tui-transformation.md`.
Tech stack: Go 1.24, Bubbletea TUI, OpenTofu IaC, Ansible config management, AWS provider.

## Build & Test

```bash
make build          # go build -o rancher-tui
make test           # go test ./...
make test-cover     # go test -coverprofile + go tool cover
make test-verbose   # go test -v ./...
make lint           # go vet ./...
make clean          # remove binary + coverage files
```

## TDD Workflow

1. Write tests first in `*_test.go` alongside the source file
2. Run `make test` — confirm the new tests fail
3. Implement the feature
4. Run `make test` — confirm all tests pass
5. Every new feature or bug fix needs tests

## Test Conventions

- **Table-driven tests** with `t.Run()` subtests
- **testify/assert** for assertions (`assert.Equal`, `assert.NoError`, `assert.True`)
- **`t.TempDir()`** for any file I/O tests (auto-cleaned)
- **No mocks for pure logic** — only mock external dependencies (network, exec)
- Follow the style in `internal/config/validation_test.go`

## Package Roles

| Package | Purpose |
|---|---|
| `internal/config/` | YAML persistence: clusters, AMIs, profiles, validation |
| `internal/core/` | Interfaces (`Provider`, `Orchestrator`), registry, shared types |
| `internal/credentials/` | AWS credential management |
| `internal/generator/` | Go `text/template` renderer for Terraform + Ansible |
| `internal/orchestrators/rke2/` | RKE2 orchestrator: playbook + inventory generation |
| `internal/orchestrators/k3s/` | K3s orchestrator: playbook + inventory generation |
| `internal/orchestrators/harvester/` | Harvester orchestrator (scaffold: config/validation, generation not implemented) |
| `internal/providers/aws/` | AWS provider: Terraform generation, EC2 outputs |
| `internal/providers/vagrant/` | Vagrant/libvirt local provider (scaffold: config/validation, generation not implemented) |
| `internal/tui/` | Bubbletea state machine, layout, header/footer |
| `internal/tui/keymap/` | nvim-style key-sequence resolution engine (pure logic, no bubbletea dep) + which-key |
| `internal/tui/views/` | TUI views: dashboard, cluster list, forms, modals |
| `internal/upgrade/` | Rancher upgrade runner (Ansible-based) |
| `internal/workflow/` | Deployment orchestration (`ModularRunner`) |
| `internal/utils/` | Zap logger initialization |

## Key Patterns

- **Provider/Orchestrator interfaces** in `internal/core/interfaces.go` — extensible via registry
- **Bubbletea state machine** in `internal/tui/root.go` — 14 states defined in `views/messages.go` (initial state is `StateDashboard`)
- **Dashboard home screen** in `internal/tui/views/dashboard.go` — Astronvim-style banner + action menu; pure `dashMenu` nav logic is unit-tested
- **nvim-style command engine** in `internal/tui/keymap/` — pure-logic `Resolver` (leader=space, `j/k`, `gg/G`, `:`, `/`, which-key); kept free of bubbletea so sequence resolution is unit-testable
- **Template rendering** via `internal/generator/renderer.go` — `Render`, `RenderString`, `RenderWithFuncs`
- **Background goroutines** for deploy/delete/upgrade — non-blocking TUI with log streaming
- **Auto-refresh** every 1 second in cluster list

## Config Files (all 0600 permissions)

| File | Content |
|---|---|
| `config.yaml` | Cluster definitions with status, IPs, timestamps |
| `cloud-credentials.yaml` | AWS access/secret key pairs |
| `profiles.yaml` | Saved infrastructure profiles |
| `amis.yaml` | AMI catalog (distro/region/AMI-ID) |

## Feature Workflow

1. Write a proposal in `feats/<name>.md`
2. Implement the feature with tests
3. Rename to `feats/x-(completed)-<name>.md` when done

## Documentation

- `docs/architecture.md` — technical architecture, packages, data flows
- `docs/product.md` — product decisions, features, version history, roadmap
