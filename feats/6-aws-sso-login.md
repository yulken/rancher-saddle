# Feature: AWS SSO (IAM Identity Center) Login

## Summary

Add AWS SSO as an authentication mode alongside static IAM keys. A user picks an SSO profile that already exists in `~/.aws/config`, logs in once from a shell (`aws sso login --profile X`, or `rancher-tui sso login`), and every subsequent `tofu`/`ansible` step runs with `AWS_PROFILE` set and no secret material written by saddle. Terraform renders `profile = "<name>"` instead of literal `access_key`/`secret_key`, which removes the three copies of long-lived keys that the app currently produces.

---

## Motivation

Credentials currently travel as plaintext through three files:

| Step | Location | Content |
|---|---|---|
| 1. Stored | `cloud-credentials.yaml` (0600) | `access_key`, `secret_key` per credential |
| 2. Copied into the cluster | `config.yaml` → `provider.config.access_key/secret_key` (`internal/tui/views/createform.go:1066-1069`) | same long-lived keys, duplicated per cluster |
| 3. Rendered into IaC | `build/<cluster>/main.tf` (`internal/providers/aws/templates/main.tf.tmpl:10-14`) | same keys again, in HCL |

Consequences:

- Rotating a key means editing every cluster that copied it (`tofu destroy` reads the rendered `main.tf`, so old builds keep working only while the old key remains valid).
- Keys live longer than the deployment they were created for. A cluster left running for a month keeps a valid key on disk for a month.
- Anyone who can read the build directory (or a backup/`scp`/paste of it) gets full account access. `build/` is gitignored, but that only covers the commit path.
- Organizations that enforce IAM Identity Center / short-lived credentials cannot use the tool at all today.

SSO is the natural fix because the credentials never need to exist in saddle's files: the AWS CLI caches a token in `~/.aws/sso/cache`, the AWS SDK (used by the Terraform provider) exchanges it for 1-hour role credentials, and refreshes those automatically while the token is valid.

Reference: the same TUI already deploys EC2 instances that users must be able to `tofu destroy` later; that path (`internal/cluster/commands.go:218,248`) inherits whatever auth the cluster was created with.

---

## Design Decisions

| Decision | Choice | Reason |
|---|---|---|
| Integration strategy | **Delegate to AWS CLI v2** (`aws sso login --profile X`), then `profile = "X"` in the provider block | Official flow, zero SDK surface in saddle, no token cache/refresh code to own. Native `ssooidc` device flow is deferred (see Alternatives) |
| Where profiles come from | **Discover** profiles already present in `~/.aws/config`; never rewrite that file | The user's AWS config is shared with their shell/CLI. Silently editing it is surprising and hard to debug |
| Credential model | Extend the existing `AWSCredential` with `type` + `profile` (rather than a parallel type) | The create-form picker, credentials list and profiles already key off credential names |
| Backward compatibility | `type` empty ⇒ `static`, exactly today's behavior; existing `cloud-credentials.yaml` keeps working untouched | No migration required to ship |
| Auth persistence per cluster | `provider.config.auth = "sso"` + `provider.config.profile = "<name>"`; never the keys | `tofu destroy` needs to know how to authenticate months later |
| Static keys in the template | **Removed in this feature**; static mode passes `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN` via `cmd.Env` | Same code path as SSO, one place to reason about env, and stops writing secrets to `main.tf` |
| Session expiry handling | Preflight `aws sts get-caller-identity` before `tofu apply`/`destroy`; typed `ErrSSOExpired` with a remediation hint | `sts:GetCallerIdentity` is always allowed and fails fast (seconds) instead of after a partial apply |
| Login UX | Outside the TUI: `aws sso login --profile X` (also `rancher-tui sso login`), then `r` in the credentials list to re-probe | The interactive device-code flow and a terminal handoff do not belong in the TUI; the screen only reports state |
| Dependency check | No version sniffing: a missing CLI or an unknown profile surfaces as a typed error from `CheckSession`/`DescribeSession` (`ErrSSOCLIUnavailable`, `ErrUnknownProfile`), and SSO credentials are only offered for profiles discovered in `~/.aws/config` | Clear error beats "exec: aws: not found" mid-deploy, without maintaining a version probe |

---

## User Flow

```
Credentials (Ctrl+X) ──> n ──> Type: AWS SSO
                               Profile:      acme-prod      (discovered from ~/.aws/config)
                               SSO Start URL: https://acme.awsapps.com/start   (read-only, from config)
                               SSO Region:    us-east-1                         (read-only, from config)
                               [ Save ]

Credentials list ──> r                (shell, beforehand: aws sso login --profile acme-prod)
                       └─ re-probes every SSO profile and re-reads the token cache
                       └─ Status: ✓ valid (acct 123456789012)
                       └─ detail line: profile acme-prod · identity center us-east-1 · token expires in 7h12m

Create form ──> Credentials: acme-prod ──> ... ──> deploy
                       ├─ preflight: "Verifying AWS credentials..." → aws sts get-caller-identity --profile acme-prod
                       └─ tofu init/apply with AWS_PROFILE=acme-prod

Delete (x) ──> reads auth=profile from config.yaml ──> tofu destroy with AWS_PROFILE
                       └─ if the token expired: "SSO session expired — run `aws sso login --profile acme-prod`"
```

Everything after `tofu` is unchanged: Ansible talks SSH, not AWS.

---

## Implementation Sketch

### 1. Credential model — `internal/credentials/credentials.go`

```go
const (
    AuthStatic = "static"
    AuthSSO    = "sso"
)

type AWSCredential struct {
    Name          string `yaml:"name"`
    Type          string `yaml:"type,omitempty"`        // "", "static", "sso"
    AccessKey     string `yaml:"access_key,omitempty"`
    SecretKey     string `yaml:"secret_key,omitempty"`
    Profile       string `yaml:"profile,omitempty"`     // ~/.aws/config profile
    SSOStartURL   string `yaml:"sso_start_url,omitempty"`
    SSORegion     string `yaml:"sso_region,omitempty"`  // Identity Center region, NOT the deploy region
    DefaultRegion string `yaml:"default_region,omitempty"`
}

func (a *AWSCredential) AuthType() string {
    if a.Type == "" { return AuthStatic } // legacy entries
    return a.Type
}

func (a *AWSCredential) Validate() error {
    // name always required
    switch a.AuthType() {
    case AuthSSO:
        if a.Profile == "" { return fmt.Errorf("SSO profile is required") }
    case AuthStatic:
        // current rules: access_key + secret_key required
    default:
        return fmt.Errorf("unknown credential type %q", a.Type)
    }
    return nil
}
```

The start URL / SSO region are informational metadata for the UI. The source of truth
for logging in remains `~/.aws/config` (`aws configure get sso_start_url --profile X`).

### 2. Profile discovery — `internal/providers/aws/sso.go` (new)

```go
type SSOProfile struct {
    Name      string
    StartURL  string
    SSORegion string
    AccountID string
    RoleName  string
}

// ListSSOProfiles parses ~/.aws/config (including sso_session blocks added in
// newer CLI versions) and returns profiles that use SSO.
func ListSSOProfiles() ([]SSOProfile, error)

// CheckSession reports whether a profile can authenticate right now. It runs
// `aws sts get-caller-identity` and maps failures to typed errors:
//   exit 0                        -> nil
//   no cached token               -> ErrSSONotLoggedIn
//   "SSO session ... expired"     -> ErrSSOExpired
//   `aws` not on PATH             -> ErrSSOCLIUnavailable
//   unknown profile               -> ErrUnknownProfile
func CheckSession(ctx context.Context, cfg map[string]interface{}) error

// DescribeSession is the display form of the same probe, used for the list
// column: "✓ valid (acct 123456789012)", "✗ expired", "✗ not logged in",
// "✗ aws cli missing", "✗ profile not found".
func DescribeSession(ctx context.Context, profile string) string

// SSOTokenExpiry answers "how long is this token good for?" without an API
// call: it reads ~/.aws/sso/cache and returns the newest access token matching
// the profile's start URL + Identity Center region.
func SSOTokenExpiry(profile SSOProfile) (expiry time.Time, found bool)
```

Parsing `~/.aws/config` is preferred over `aws configure list-profiles` because it
needs no subprocess and is trivially unit-testable with fixture INI files. Support both
the legacy `sso_start_url` form and the `sso_session = <name>` form.

### 3. Terraform provider block — `internal/providers/aws/templates/main.tf.tmpl`

```hcl
provider "aws" {
  region = "{{.Region}}"
{{- if .Profile }}
  profile = "{{.Profile}}"
{{- else }}
  # credentials come from AWS_* env vars set by the runner — never from this file
{{- end }}
}
```

`AWSConfig` (`internal/providers/aws/config.go`) gains:

```go
AuthMode string // "static" | "sso"
Profile  string
```

populated from `auth` / `profile` in `FromMap`, with `AuthMode` defaulting to `static`.

Note the deploy region (`region`) and the Identity Center region (`sso_region`) are
different things and must not be conflated — a common source of confusing 403s.

### 4. Environment plumbing (the part that actually makes it work)

`internal/providers/aws/auth.go` (new):

```go
// EnvFor returns the process environment for tofu/ansible invocations.
func EnvFor(cfg map[string]interface{}) []string {
    env := os.Environ()
    // Ambient static credentials take precedence over `profile` in the AWS SDK,
    // so they must be scrubbed whenever we authenticate by profile.
    env = unset(env, "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN")

    switch cfg["auth"] {
    case "sso":
        env = append(env, "AWS_PROFILE="+cfg["profile"].(string))
    case "static", nil:
        env = append(env, "AWS_ACCESS_KEY_ID="+cfg["access_key"].(string))
        env = append(env, "AWS_SECRET_ACCESS_KEY="+cfg["secret_key"].(string))
    }
    return env
}
```

The call sites that must change:

| Call site | Change |
|---|---|
| `internal/workflow/runner_new.go:262` `runCommand` | takes `env []string`; call sites pass `providerAuthEnv(providerConfig)` (`:104`) for `tofu init/apply` and `nil` for Ansible |
| `internal/cluster/commands.go:218,248` `tofu destroy` | preflight with `CheckSession`, then `cmd.Env = aws.EnvFor(cluster.Provider.Config)` |
| `internal/tui/views/deletemodal.go:182,198` `tofu destroy` | same preflight and env for the TUI delete path |
| `internal/providers/aws/validator.go` | reject `auth=sso` with an empty profile, and static auth with missing keys |

Ansible does **not** get `AWS_*` from saddle. It authenticates over SSH only, and AWS
credentials in a playbook environment are an unnecessary privilege that can leak through
`debug` tasks, `environment:` blocks, or a verbose run. `EnvFor` is applied to `tofu`
invocations exclusively.

### 5. Preflight and error surfacing

Before `tofu init` (and again before `destroy`):

```go
if err := aws.CheckSession(ctx, providerConfig); err != nil {
    return fmt.Errorf("%w: %w", aws.ErrSSOExpired, err)
}
// The account number is not logged here; it is shown in the credentials list
// status column ("✓ valid (acct …)"). The preflight only prints
// "Verifying AWS credentials..." and stops on error.
```

```go
var ErrSSOExpired = errors.New("SSO session expired")
```

The TUI renders it as an actionable message rather than a bare tofu failure:

```
✗ SSO session for profile "acme-prod" expired.
  Run: aws sso login --profile acme-prod
```

Parsing stderr for expiry is brittle; the preferred implementation runs
`aws sts get-caller-identity` and treats *any* auth failure as "session unusable",
with the raw stderr appended to the log panel for diagnosis.

### 6. TUI changes

**`internal/tui/views/credentialsform.go`** — add a Type selector at index 0 and swap the
middle fields based on it. Static fields stay byte-for-byte compatible with today's form.

**`internal/tui/views/credentialslist.go`** — replace the `Access Key` column with `Status`:

```
Name             Provider   Region     Status
acme-prod        AWS SSO    us-east-1  ✓ valid (acct 123456789012)
sandbox          AWS SSO    us-west-2  ✗ expired
legacy-static    AWS        us-east-1  keys
```

A detail line under the table shows the selected SSO credential's cached token
lifetime (`profile acme-prod · identity center us-east-1 · token expires in 7h12m`),
read from `~/.aws/sso/cache` without an API call, and `r` reloads the credentials
file and re-probes the sessions.

The in-TUI login key (`tea.ExecProcess` handing the terminal to `aws sso login`) was
prototyped and dropped: the login flow belongs in a shell, where the user already runs
it, and the screen only needs to report the resulting state. Login stays available as
`aws sso login --profile X` and `rancher-tui sso login`.

**`internal/tui/views/createform.go`** — the credential picker already lists names from
`cloud-credentials.yaml` (`createform.go:686`), so SSO entries appear with no extra work.
On submit, branch on `awsCred.AuthType()`:

```go
cfg := map[string]interface{}{
    "region": region, /* ... */
    "auth":    "sso",
    "profile": awsCred.Profile,
}
// static entries keep writing access_key/secret_key for backward compatibility
```

Optional (phase 2): warn when the selected SSO session is already expired instead of
letting preflight fail. Phase 1 fails at preflight with the same remediation text.

### 7. CLI

Add a `login` command to the cobra root so CI/headless users are not forced through the TUI:

```bash
rancher-tui sso login --profile acme-prod [--device-code]
rancher-tui sso status [--profile acme-prod]
```

Both are thin wrappers over the same helpers the credentials list uses:
`ListSSOProfiles` + `DescribeSession` for status. `sso login` is the only login path —
the TUI does not drive it.

---

## Files to Create / Modify

| File | Action |
|---|---|
| `internal/credentials/credentials.go` | **MODIFY** — `Type`, `Profile`, `SSOStartURL`, `SSORegion`; type-aware `Validate()`; `AuthType()` |
| `internal/credentials/credentials_test.go` | **NEW** — validation matrix for static/sso/legacy/unknown |
| `internal/providers/aws/sso.go` | **NEW** — `ListSSOProfiles`, `FindSSOProfile`, `CheckSession`, `DescribeSession`, `SSOTokenExpiry`, `ErrSSOExpired` |
| `internal/providers/aws/sso_test.go` | **NEW** — INI fixtures (legacy + `sso_session` form), stubbed CLI runner for status mapping, token-cache fixtures |
| `internal/providers/aws/auth.go` | **NEW** — `EnvFor` (config map), `EnvForStatic` (legacy `model.Config` path), `unset` |
| `internal/providers/aws/auth_test.go` | **NEW** — env scrubbing/injection per auth mode |
| `internal/providers/aws/config.go` | **MODIFY** — `AuthMode`, `Profile` in `FromMap` |
| `internal/providers/aws/templates/main.tf.tmpl` | **MODIFY** — conditional `profile`, drop literal keys |
| `internal/providers/aws/validator.go` | **MODIFY** — validate auth mode/profile coherence |
| `internal/generator/tofu.go` | **MODIFY** — legacy `GenerateTofu` template also renders literal keys; remove them and rely on `EnvFor` |
| `internal/workflow/runner_new.go` | **MODIFY** — `runCommand` sets `cmd.Env`; preflight in `RunWithBuildDir` |
| `internal/cluster/commands.go` | **MODIFY** — `DeleteCluster` sets `cmd.Env` + preflight |
| `internal/tui/views/credentialsform.go` | **MODIFY** — type selector, SSO fields |
| `internal/tui/views/credentialslist.go` | **MODIFY** — status column, token lifetime, `r` refresh |
| `internal/tui/views/createform.go` | **MODIFY** — write `auth`/`profile` instead of keys |
| `main.go` | **MODIFY** — `sso login` / `sso status` subcommands |
| `docs/architecture.md`, `README.md` | **MODIFY** — document auth modes and the AWS CLI prerequisite |

No new Go module dependencies in phase 1.

---

## What Is Actually Sent Where

Credentials never travel to the provisioned servers. This matters for the migration story,
so it is worth stating explicitly:

| Artifact | Where it goes | Contains AWS credentials? |
|---|---|---|
| `build/<cluster>/main.tf` | local disk | today: **yes** (both keys). After this feature: no |
| AWS API calls from `tofu` | HTTPS to the EC2/SSO endpoints | signed requests only — the secret key is used locally to compute a SigV4 signature and is never transmitted |
| EC2 nodes | n/a | no. The template attaches no `iam_instance_profile`, and no orchestrator template references `AWS_*`, `cloud-provider`, or `ec2` |
| Ansible inventory/playbooks | SSH to the nodes | no |
| `~/.aws/sso/cache` | local disk (managed by AWS CLI) | SSO access token, not IAM keys |
| Terraform provider process | memory | role credentials (1h), refreshed automatically |
| `config.yaml` | local, gitignored | today: both keys per cluster. After this feature: only `auth`/`profile` for SSO clusters |

Consequence: revoking a key does **not** break running clusters, Rancher, workloads, or
upgrades (which run over SSH). It only blocks AWS control-plane operations launched from
the operator machine.

## Impact on Existing Clusters

| Operation | Needs AWS credentials? |
|---|---|
| Running clusters, workloads, Rancher UI | No |
| `saddle list` | No — reads local `config.yaml` |
| Rancher upgrade (`u`) | No — Ansible over SSH |
| Re-`tofu apply` (retry) | Yes |
| `tofu destroy` (cluster delete) | **Yes** |

Two migration traps to document in the release notes:

1. **Static keys hardcoded in `main.tf` win over environment variables.** The AWS provider's
   resolution order is provider-block static credentials → env vars → shared config. An
   existing build directory with revoked keys rendered into it cannot be rescued with
   `AWS_PROFILE`; the file must be re-rendered (or edited). This is the strongest argument
   for removing keys from the template now.
2. **`destroy` needs permission in the same account, not the same key.** Terraform state is
   local (no remote backend), so there is nothing else to authenticate against, but the SSO
   permission set must grant at least the permissions the old IAM user had. Do not revoke
   the old keys until existing build directories have been re-rendered or the clusters
   deleted.

---

## Migration and Backward Compatibility

- Existing `cloud-credentials.yaml` entries have no `type` ⇒ treated as static. Nothing breaks.
- Existing `config.yaml` clusters keep `access_key`/`secret_key`; `EnvFor` still exports them,
  so `tofu destroy` on old clusters keeps working unchanged.
- New clusters created with SSO store only `auth` + `profile`.
- Clusters created with static keys after this change still need their keys in `config.yaml`
  (delete path depends on them). Cleaning that up is out of scope; it shrinks naturally as
  users move to SSO.
- `main.tf` no longer contains any credential in either mode. Existing build directories still do;
  they are gitignored and get removed on `tofu destroy`, but a fresh apply rewrites them clean.

---

## Testing Plan

Following `CLAUDE.md` (TDD, table-driven, `testify`, `t.TempDir()`):

| Test | What it asserts |
|---|---|
| `credentials.Validate` table | legacy empty type = static; static requires both keys; sso requires profile only; unknown type errors |
| `AWSConfig.FromMap` | `auth=sso` + `profile` populate `AuthMode`/`Profile`; absent `auth` defaults to static; keys ignored in sso mode |
| Template render | sso output contains `profile = "..."` and **no** `access_key`; static output contains no keys either |
| `EnvFor` | sso: `AWS_PROFILE` added, static vars scrubbed even when present in the parent env; static: keys exported; result never contains duplicates |
| `ListSSOProfiles` | legacy `sso_start_url` block, `sso_session` indirection, mixed non-SSO profiles, empty file, missing file |
| `CheckSession` | fake exec runner: exit 0 ⇒ nil; stderr "SSO session ... expired" ⇒ `ErrSSOExpired`; other stderr ⇒ wrapped generic error |
| `DeleteCluster` env | unit test on the command builder (refactor the exec into a small injectable helper) — **not implemented**; the CLI and TUI destroy paths are still duplicated and untested |
| TUI form | type switch shows/hides the right fields (covered); submit builds sso vs static provider config — **not implemented**, `createCluster` has no test |
| Manual | fresh profile → login → deploy → `tofu destroy`; then expire the token (or use an invalid profile) and confirm the remediation message appears before any AWS call |

---

## Alternatives Considered

**Native device flow via `aws-sdk-go-v2` (`ssooidc` + `sso` + `sts`).**
Implemented entirely in Go: `RegisterClient` → `StartDeviceAuthorization` → `CreateToken`,
cache the token, call `GetRoleCredentials` on demand. Pros: no AWS CLI dependency, full
control of the UX. Cons: we re-implement token caching, refresh, and expiry semantics that
AWS already ships; adds ~4 modules and a background refresh concern to a TUI; the CLI's
cache format is de facto standard and users may already have valid sessions there.
Deferred to a phase 2, only if "AWS CLI must not be installed" becomes a requirement.

**`credential_process` helper.**
Would let `tofu` obtain credentials lazily, but requires writing a profile into
`~/.aws/config` and a helper binary. More moving parts for the same result.

**Keep keys, add SSO only in the UI.**
Rejected: leaves the plaintext copies in place and doubles the auth paths instead of
unifying them behind `EnvFor`.

---

## Pros

- No long-lived secrets written by saddle — in SSO mode, nothing is written at all.
- Key rotation stops mattering for SSO users; Identity Center policy applies automatically.
- Org policies that forbid IAM users become usable ("only SSO" shops can adopt the tool).
- `main.tf` stops carrying credentials in *both* modes, closing an existing leak path.
- Auth choice is recorded per cluster, so `destroy` months later still knows what to do.
- One env-construction helper for apply and destroy prevents the classic
  "worked on create, failed on destroy" divergence.

## Cons / Caveats

- Requires AWS CLI v2 on the machine. Must be a documented prerequisite and a checked
  preflight, otherwise unsupported users hit a late, opaque failure.
- **SSO token lifetime is bounded by Identity Center (default 8h).** The 1-hour role
  credentials are renewed transparently by the provider while the token is valid, but
  renewal after token expiry requires a human (`aws sso login`). Two consequences the tool
  cannot fully prevent: (a) a deploy started with less token lifetime remaining than it
  takes to finish can fail *mid-apply*, when role credentials are refreshed; (b) `tofu
  destroy` months later always needs a fresh login. Mitigations: preflight fail-fast, a
  warning when `expiresAt` is close relative to a typical deploy duration (phase 2), and
  idempotent re-apply after re-login (see `feats/5-deployment-checkpoints.md`).
- `tofu destroy` on an SSO cluster can now fail for an auth reason — previously keys were
  embedded in the build dir. The remediation message must be impossible to miss.
- Two auth modes mean the template has a branch and the tests double. Accepted for the
  migration window; static mode can be deprecated later.
- Profiles live outside the repo, so "it works on my machine" can depend on
  `~/.aws/config`. `sso status` prints the resolved status per profile (and the account,
  once authenticated), which makes that visible; it does not print the start URL, Identity
  Center region or role name yet.
- Reading `~/.aws/config` must handle the `[profile x]` vs `[sso-session x]` and
  `[default]` quirks; a hand-rolled INI parser needs its own tests.

---

## Phases

| Phase | Scope |
|---|---|
| **1 (this doc)** | SSO credential type, discovery, login via CLI, `profile` in template, `EnvFor` for apply+destroy, preflight, status column, `sso login`/`sso status` commands, tests, docs |
| 2 | Expired-session warning in the create form; session status caching to avoid one `sts` call per screen refresh; pre-deploy warning when the token expires sooner than a typical deploy |
| 3 | Native `ssooidc` device flow (drop the CLI dependency), account/role picker via `sso:ListAccounts`/`ListAccountRoles`, background token refresh |

---

## Implementation Status

Phase 1 landed in this branch:

- `AWSCredential` gained `type`/`profile`/SSO metadata; legacy entries default to static.
- `internal/providers/aws/{auth,sso}.go`: env construction, profile discovery
  (`~/.aws/config`, both `sso_start_url` and `sso_session` forms), STS preflight,
  error classification (`ErrSSOExpired`, `ErrSSONotLoggedIn`, `ErrSSOCLIUnavailable`,
  `ErrUnknownProfile`).
- Terraform templates (provider + legacy generator) no longer render any credential.
- `ModularRunner` and both destroy paths (`cluster/commands.go`, TUI delete modal) set
  the provider environment and run the preflight.
- Credentials form has a type selector; list has a status column, a detail line with
  the cached token lifetime (`~/.aws/sso/cache`, newest token per start URL + region,
  no API call), and `r` to refresh. The screen otherwise probes the session only when
  it is opened, so a login performed in another terminal went unnoticed.
- No in-TUI login: the terminal handoff (`tea.ExecProcess` → `aws sso login`) was
  prototyped and dropped as out of scope. Log in from a shell with
  `aws sso login --profile X` (or `rancher-tui sso login`) and press `r`.
- On first run (no `cloud-credentials.yaml`), SSO profiles from `~/.aws/config` are
  imported automatically, so a user who already ran `aws sso login` sees the profiles
  without registering them by hand. Deleted entries are not re-imported.
- `rancher-tui sso login|status` CLI commands.
- Static (pre-SSO) credentials verified end-to-end: a real deploy with a legacy
  `cloud-credentials.yaml` (no `type`) and a cluster config without `auth` completed
  successfully after the change, confirming backward compatibility.
- Tests: credentials validation, env scrubbing, profile discovery, error mapping,
  template rendering, auth-mode validation, form field logic, token-cache matching
  (newest entry wins, stale entries ignored), credentials-list detail line and `r` refresh.

Not implemented yet — phase 2: expired-session warning in the create form, session status
caching, pre-deploy warning when the token expires sooner than a typical deploy.
Not implemented yet — test plan items: `DeleteCluster` env/preflight (the TUI and CLI
destroy paths are duplicated and untested; extracting an injectable helper is a
prerequisite), and the create-form submit path that writes `auth`/`profile`.
Not implemented yet — phase 3: native `ssooidc` flow.

Known gap in coverage: `provider.config.auth`/`profile` persistence has no test at all
(neither `ToModernConfig`/`FromModernConfig` nor the create-form submit), even though
`tofu destroy` months later depends on it.

---

## Open Questions

1. **AWS CLI version floor** — v2 only (needed for `aws sso login` and the modern
   `sso_session` blocks), or best-effort v1 support? Recommend v2 only. Still open:
   nothing in the code checks the version today; a missing CLI is only reported as
   `ErrSSOCLIUnavailable` when a session probe runs. If v1 ever needs to be rejected
   explicitly, that check belongs in `CheckSession`, not in a startup preflight.
2. **Should saddle manage `~/.aws/config`?** Recommend no for phase 1. If added later, it
   must be opt-in and print a diff before writing.
3. **Deprecate static keys?** Keep indefinitely for other clouds/edge cases, or announce a
   removal date once SSO is battle-tested? Recommend keep, document SSO as preferred.
4. **Multiple roles per account** — phase 1 assumes one profile = one account/role, which is
   how `~/.aws/config` already models it. Confirm that is enough for the target orgs.
