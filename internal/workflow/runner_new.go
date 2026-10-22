package workflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/utils"
	"go.uber.org/zap"
)

// ModularRunner creates a new workflow runner with the modular architecture
type ModularRunner struct {
	Config       *config.Config
	Registry     *core.Registry
	Logger       *zap.Logger
	Provider     core.Provider
	Orchestrator core.Orchestrator
}

// NewModularRunner creates a new runner from configuration
func NewModularRunner(cfg *config.Config, registry *core.Registry) (*ModularRunner, error) {
	logger, err := utils.InitLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	// Get provider
	provider, err := registry.GetProvider(cfg.GetProviderType())
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// Get orchestrator
	orchestrator, err := registry.GetOrchestrator(cfg.GetOrchestratorType())
	if err != nil {
		return nil, fmt.Errorf("failed to get orchestrator: %w", err)
	}

	return &ModularRunner{
		Config:       cfg,
		Registry:     registry,
		Logger:       logger,
		Provider:     provider,
		Orchestrator: orchestrator,
	}, nil
}

// Run executes the deployment workflow
func (r *ModularRunner) Run() error {
	return r.RunWithBuildDir("build")
}

// RunWithBuildDir executes the deployment workflow with a custom build directory
func (r *ModularRunner) RunWithBuildDir(buildDir string) error {
	ctx := context.Background()

	// Ensure build directory exists
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}

	// Merge common config with provider config
	providerConfig := r.mergeProviderConfig()

	// Preflight: validate provider prerequisites/config before doing any work,
	// so missing tooling (e.g. vagrant/libvirt) fails fast with a clear message.
	if err := r.Provider.Validate(providerConfig); err != nil {
		return fmt.Errorf("provider validation failed: %w", err)
	}

	// Preflight: expired SSO sessions (or missing AWS CLI) fail here in seconds,
	// instead of mid-apply once the provider needs to refresh credentials.
	if checker, ok := r.Provider.(core.SessionChecker); ok {
		fmt.Printf("Verifying %s credentials...\n", r.Provider.Name())
		if err := checker.CheckSession(ctx, providerConfig); err != nil {
			return fmt.Errorf("authentication preflight failed: %w", err)
		}
	}

	// Step 1: Generate infrastructure code
	fmt.Printf("Generating %s infrastructure configuration...\n", r.Provider.Name())

	if err := r.Provider.GenerateInfrastructure(ctx, providerConfig, buildDir); err != nil {
		return fmt.Errorf("failed to generate infrastructure: %w", err)
	}

	// Custom apply path: providers that do not fit the OpenTofu pipeline (e.g.
	// Vagrant/Harvester's two-phase PXE boot) implement ProviderWithCustomApply.
	// We still let the orchestrator render its playbook + inventory first, then
	// hand the apply phase to the provider and skip the standard steps 2-8.
	if cap, ok := r.Provider.(core.ProviderWithCustomApply); ok {
		return r.runCustomApply(ctx, cap, providerConfig, buildDir)
	}

	// Step 2: Initialize infrastructure tool (Terraform/Tofu)
	fmt.Println("Initializing OpenTofu...")
	authEnv := r.providerAuthEnv(providerConfig)
	if err := r.runCommand("tofu", []string{"init"}, buildDir, authEnv); err != nil {
		return err
	}

	// Step 3: Apply infrastructure
	fmt.Println("Applying OpenTofu plan (this may take a while)...")
	if err := r.runCommand("tofu", []string{"apply", "-auto-approve"}, buildDir, authEnv); err != nil {
		return err
	}

	// Step 4: Get infrastructure outputs
	fmt.Println("Fetching instance information...")
	outputs, err := r.Provider.GetOutputs(ctx, buildDir)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure outputs: %w", err)
	}

	if len(outputs.InstanceIPs) == 0 {
		return fmt.Errorf("no instances created")
	}

	fmt.Printf("Created %d instance(s):\n", len(outputs.InstanceIPs))
	for i := range outputs.InstanceIPs {
		dns := ""
		if i < len(outputs.InstanceDNSNames) {
			dns = outputs.InstanceDNSNames[i]
		}
		fmt.Printf("  [%d] IP: %s, DNS: %s\n", i+1, outputs.InstanceIPs[i], dns)
	}

	// Step 5: Generate orchestrator playbook
	fmt.Printf("Generating %s playbook...\n", r.Orchestrator.Name())

	// Merge common config with orchestrator config
	orchestratorConfig := r.mergeOrchestratorConfig()

	if err := r.Orchestrator.GeneratePlaybook(ctx, orchestratorConfig, buildDir); err != nil {
		return fmt.Errorf("failed to generate playbook: %w", err)
	}

	// Step 6: Generate inventory
	fmt.Println("Generating inventory...")
	if err := r.Orchestrator.GenerateInventory(ctx, outputs, orchestratorConfig, buildDir); err != nil {
		return fmt.Errorf("failed to generate inventory: %w", err)
	}

	// Step 7: Wait for SSH
	fmt.Println("Waiting for SSH to be available on all instances...")
	if err := r.waitForSSH(outputs.InstanceIPs); err != nil {
		return fmt.Errorf("failed waiting for SSH: %w", err)
	}

	// Step 8: Run playbook
	fmt.Println("Running Ansible Playbook...")
	if err := r.runCommand("ansible-playbook", []string{"-i", "hosts.ini", "site.yml"}, buildDir, nil); err != nil {
		return err
	}

	// Step 9: Display success message
	r.displaySuccess(outputs)

	return nil
}

// runCustomApply drives the deployment for providers that implement
// ProviderWithCustomApply (e.g. Vagrant/Harvester). The orchestrator still owns
// the playbook + inventory generation; the provider owns the actual apply.
func (r *ModularRunner) runCustomApply(ctx context.Context, provider core.ProviderWithCustomApply, providerConfig map[string]interface{}, buildDir string) error {
	orchestratorConfig := r.mergeOrchestratorConfig()

	// Bridge the cluster VIP from the orchestrator config into the provider
	// config so CustomApply can poll it for cluster readiness.
	if vip, ok := orchestratorConfig["vip"]; ok {
		providerConfig["vip"] = vip
	}

	// Planned outputs are derived from config (deterministic/static IPs) so the
	// inventory can be rendered before any VM is booted.
	planned, err := provider.PlannedOutputs(providerConfig)
	if err != nil {
		return fmt.Errorf("failed to plan outputs: %w", err)
	}

	// Generate the orchestrator playbook (e.g. PXE server setup).
	fmt.Printf("Generating %s playbook...\n", r.Orchestrator.Name())
	if err := r.Orchestrator.GeneratePlaybook(ctx, orchestratorConfig, buildDir); err != nil {
		return fmt.Errorf("failed to generate playbook: %w", err)
	}

	// Generate the inventory from the planned outputs.
	fmt.Println("Generating inventory...")
	if err := r.Orchestrator.GenerateInventory(ctx, planned, orchestratorConfig, buildDir); err != nil {
		return fmt.Errorf("failed to generate inventory: %w", err)
	}

	// Hand the apply phase to the provider (two-phase vagrant boot + PXE install).
	fmt.Printf("Applying %s infrastructure (this may take a while)...\n", r.Provider.Name())
	outputs, err := provider.CustomApply(ctx, providerConfig, buildDir, os.Stdout)
	if err != nil {
		return fmt.Errorf("custom apply failed: %w", err)
	}

	if len(outputs.InstanceIPs) == 0 {
		return fmt.Errorf("no instances created")
	}

	r.displaySuccess(outputs)
	return nil
}

// mergeProviderConfig merges common config with provider-specific config
func (r *ModularRunner) mergeProviderConfig() map[string]interface{} {
	merged := make(map[string]interface{})

	// Copy provider config
	for k, v := range r.Config.ProviderConfig {
		merged[k] = v
	}

	// Add common fields
	merged["cluster_name"] = r.Config.ClusterName
	merged["ssh_key_name"] = r.Config.SSHKeyName
	merged["node_prefix"] = r.Config.NodePrefix
	merged["instance_count"] = r.Config.InstanceCount

	return merged
}

// mergeOrchestratorConfig merges common config with orchestrator-specific config
func (r *ModularRunner) mergeOrchestratorConfig() map[string]interface{} {
	merged := make(map[string]interface{})

	// Copy orchestrator config
	for k, v := range r.Config.OrchestratorConfig {
		merged[k] = v
	}

	// Add common fields
	merged["cluster_name"] = r.Config.ClusterName
	merged["node_prefix"] = r.Config.NodePrefix
	merged["instance_count"] = r.Config.InstanceCount
	merged["ssh_private_key_path"] = r.Config.SSHPrivateKeyPath
	merged["ssh_user"] = r.Config.SSHUser

	return merged
}

// providerAuthEnv returns the process environment the provider needs for its
// CLI tools, or nil when the provider has no opinion (inherit the parent env).
func (r *ModularRunner) providerAuthEnv(providerConfig map[string]interface{}) []string {
	if ep, ok := r.Provider.(core.AuthEnvProvider); ok {
		return ep.AuthEnv(providerConfig)
	}
	return nil
}

// runCommand executes a command. A nil env inherits the parent environment.
func (r *ModularRunner) runCommand(name string, args []string, dir string, env []string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	r.Logger.Info("Executed command",
		zap.String("command", name),
		zap.Strings("args", args),
		zap.String("output", outputStr),
	)

	if err != nil {
		r.Logger.Error("Command failed",
			zap.Error(err),
			zap.String("command", name),
			zap.Strings("args", args),
		)

		os.MkdirAll("logs", 0755)
		errorFile := filepath.Join("logs", fmt.Sprintf("%s_error.log", name))
		os.WriteFile(errorFile, output, 0644)

		return &CommandError{
			Command: name,
			Args:    args,
			Output:  outputStr,
			Err:     err,
		}
	}

	return nil
}

// waitForSSH waits for SSH to be available on all instances. Preflights the
// key file so a missing/typo'd path fails immediately with a clear message
// instead of after 5 minutes of opaque retries, and surfaces the actual ssh
// stderr on the first/last attempts so the user can debug auth failures.
func (r *ModularRunner) waitForSSH(ips []string) error {
	maxRetries := 30
	retryDelay := 10

	keyPath, err := config.ExpandPath(r.Config.SSHPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("ssh_private_key_path %q: %w", r.Config.SSHPrivateKeyPath, err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("ssh key not found at %q (expanded from %q): %w",
			keyPath, r.Config.SSHPrivateKeyPath, err)
	}

	for i, ip := range ips {
		fmt.Printf("  [%d/%d] Waiting for SSH on %s...\n", i+1, len(ips), ip)

		for retry := 0; retry < maxRetries; retry++ {
			cmd := exec.Command("ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				"-o", "BatchMode=yes",
				"-i", keyPath,
				fmt.Sprintf("%s@%s", r.Config.SSHUser, ip),
				"echo 'SSH Ready'",
			)

			output, err := cmd.CombinedOutput()
			if err == nil && strings.Contains(string(output), "SSH Ready") {
				fmt.Printf("  [%d/%d] ✓ SSH ready on %s\n", i+1, len(ips), ip)
				break
			}

			if retry < maxRetries-1 {
				fmt.Printf("  [%d/%d] SSH not ready on %s, retrying in %ds (attempt %d/%d)\n",
					i+1, len(ips), ip, retryDelay, retry+1, maxRetries)
				// Show the actual ssh output on the first attempt and every 5th
				// retry so the user can see the underlying reason (wrong user,
				// missing key, permission denied, etc.) instead of just "not ready".
				if retry == 0 || retry%5 == 4 {
					trimmed := strings.TrimSpace(string(output))
					if trimmed != "" {
						fmt.Printf("      ssh: %s\n", firstNonEmptyLine(trimmed))
					}
				}
				time.Sleep(time.Duration(retryDelay) * time.Second)
			} else {
				return fmt.Errorf("SSH not available on %s after %d attempts: %s",
					ip, maxRetries, firstNonEmptyLine(strings.TrimSpace(string(output))))
			}
		}
	}

	fmt.Println("✓ All instances are ready for provisioning")
	return nil
}

// firstNonEmptyLine returns the first non-empty line of s, trimmed.
// Helps keep the SSH-progress output tidy by collapsing OpenSSH's
// multiline warnings/errors to the most informative line.
func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// displaySuccess displays the success message
func (r *ModularRunner) displaySuccess(outputs *core.InfrastructureOutputs) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🎉 Deployment Complete!")
	fmt.Println(strings.Repeat("=", 70))

	// Get primary DNS/IP for access
	primaryAccess := ""
	if len(outputs.InstanceDNSNames) > 0 {
		primaryAccess = outputs.InstanceDNSNames[0]
	} else if len(outputs.InstanceIPs) > 0 {
		primaryAccess = outputs.InstanceIPs[0]
	}

	// Check if Rancher is deployed
	deployRancher := false
	if v, ok := r.Config.OrchestratorConfig["deploy_rancher"].(bool); ok {
		deployRancher = v
	}

	if deployRancher && primaryAccess != "" {
		fmt.Printf("\n📍 Rancher Dashboard: https://%s/dashboard\n", primaryAccess)
		fmt.Printf("🔑 Username: admin\n")
		fmt.Printf("🔑 Password: admin\n")
	}

	fmt.Printf("\n📋 Cluster Details:\n")
	fmt.Printf("   - Provider: %s\n", r.Config.Provider)
	fmt.Printf("   - Orchestrator: %s\n", r.Config.Orchestrator)
	fmt.Printf("   - Nodes: %d\n", len(outputs.InstanceIPs))

	// Display orchestrator version if available
	if version, ok := r.Config.OrchestratorConfig["rke2_version"].(string); ok {
		fmt.Printf("   - RKE2 Version: %s\n", version)
	}
	if version, ok := r.Config.OrchestratorConfig["rancher_version"].(string); ok {
		fmt.Printf("   - Rancher Version: %s\n", version)
	}

	fmt.Printf("\n📝 Full logs: logs/deployment.log\n")
	fmt.Println(strings.Repeat("=", 70) + "\n")
}
