# Feature: Transform rancher-saddle into rancher-tui

## Summary

Grow this project from a RKE2/K3s + Rancher-on-AWS deployer into **rancher-tui**: a
cohesive TUI for creating and managing the whole Rancher product family (Rancher,
Harvester, Longhorn, observability, RKE2, K3s) across cloud and local environments,
for both production and test use. The home screen is an Astronvim/Tornado-style
dashboard and navigation uses nvim-style keybindings.

Because this is large, it ships in **phases**. This document tracks the overall plan;
Phase 1 is implemented.

---

## Motivation

- One terminal workflow for the full product family, not just RKE2/K3s.
- A fast, keyboard-first (nvim) experience with a dashboard overview.
- Local Harvester for dev/test via Vagrant + libvirt/KVM, alongside cloud installs.
- An extensible foundation (Provider/Orchestrator registry + command engine) so new
  products slot in without reworking the UI.

---

## Phase 1 — Foundation (implemented)

1. **Full rename** `github.com/Felipalds/rancher-saddle` → `rancher-tui` (module path,
   all imports, binary `saddle` → `rancher-tui`, cobra cmd `corral` → `rancher-tui`,
   header title/version).
2. **Dashboard home screen** (`internal/tui/views/dashboard.go`, `StateDashboard`):
   ASCII banner, centered action menu, recent-cluster summary, nvim hint footer. It is
   the initial state; `esc` returns here from any view.
3. **nvim-style command engine** (`internal/tui/keymap/`): a pure-logic `Resolver`
   (no bubbletea dependency) for multi-key sequences, leader expansion (leader = space),
   timeout reset, and modal switching (normal / `:` command / `/` search), plus a
   which-key popup renderer. Bindings: `j/k`, `gg`, `G`, `:`, `/`, `?`, `<leader>ff`,
   `<leader>q`.
4. **Harvester + Vagrant scaffold**:
   - `core.OrchestratorHarvester`, `core.ProviderVagrant`
   - `internal/orchestrators/harvester/` — `HarvesterConfig` (install target
     cloud|vagrant, libvirt backend), validation, form fields; generation returns
     `ErrNotImplemented`.
   - `internal/providers/vagrant/` — `VagrantConfig` (libvirt/KVM), validation, form
     fields; generation returns `ErrNotImplemented`.
   - Registered in `main.go`; selectable in the create form behind a not-implemented guard.

All Phase 1 work is TDD-first (tests written before implementation), table-driven with
testify, following `internal/config/validation_test.go`.

### Phase 1 TDD checklist
- [x] `internal/tui/keymap/keymap_test.go` — resolve, multi-key, leader, timeout, modes, which-key, no-collisions
- [x] `internal/tui/keymap/whichkey_test.go` — popup contains every candidate
- [x] `internal/tui/views/dashboard_test.go` — menu nav wraparound, selection, enter→StateChange, `AppState.String`
- [x] `internal/orchestrators/harvester/config_test.go` + `orchestrator_test.go`
- [x] `internal/providers/vagrant/config_test.go` + `provider_test.go`
- [x] `internal/core/registration_integration_test.go` — fresh-registry registration

---

## Later phases (not implemented)

- **Phase 2 — Harvester installs:** full cloud + local (Vagrant/libvirt, iPXE) flows.
  See `feats/2-harvester-vagrant.md` for the detailed design.
- **Longhorn:** distributed block storage management (Helm/Ansible on an existing cluster).
- **Observability:** Rancher monitoring / Prometheus + Grafana stack.
- **Environments:** production vs. test presets.

### Deferred in Phase 1 (conscious)
- The legacy `internal/model.Config` vs `internal/config.Config` split, and the legacy
  `cmd/tui.go` `Model`/`MenuModel` vs `RootModel` stacks, are left intact (the rename
  only touched their imports).
- On-disk directory stays `rancher-saddle/`; the Go module path is what was renamed.
- The nvim engine is additive — it resolves sequences alongside the existing per-view
  key handling rather than replacing it.

---

## Verification

```bash
make build   # produces ./rancher-tui
make test    # all packages green, incl. keymap/dashboard/harvester/vagrant
make lint    # go vet clean
./rancher-tui list-orchestrators   # includes harvester
./rancher-tui list-providers       # includes vagrant
```

Manual: launch `./rancher-tui` from the repo root → dashboard renders; `j/k` move the
menu, `enter` opens a view, `esc` returns to the dashboard; `?` shows help with nvim keys.
