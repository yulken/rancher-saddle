package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Felipalds/rancher-tui/internal/generator"
	"github.com/Felipalds/rancher-tui/internal/model"
	"github.com/Felipalds/rancher-tui/internal/providers/aws"
	"github.com/Felipalds/rancher-tui/internal/utils"
	"go.uber.org/zap"
)

type Runner struct {
	Config *model.Config
	Logger *zap.Logger
}

// CommandError represents a detailed error from a failed command execution
type CommandError struct {
	Command string
	Args    []string
	Output  string
	Err     error
}

func (e *CommandError) Error() string {
	var sb strings.Builder
	sb.WriteString("\n╔══════════════════════════════════════════════════════════════════╗\n")
	fmt.Fprintf(&sb, "║ COMMAND FAILED: %s %s\n", e.Command, strings.Join(e.Args, " "))
	sb.WriteString("╠══════════════════════════════════════════════════════════════════╣\n")
	fmt.Fprintf(&sb, "║ Error: %v\n", e.Err)
	sb.WriteString("╠══════════════════════════════════════════════════════════════════╣\n")
	sb.WriteString("║ OUTPUT:\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════════╝\n\n")

	// Add output with proper formatting
	if e.Output != "" {
		lines := strings.Split(e.Output, "\n")
		for _, line := range lines {
			fmt.Fprintf(&sb, "  %s\n", line)
		}
	} else {
		sb.WriteString("  (no output)\n")
	}

	sb.WriteString("\n╔══════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║ Full logs available in: logs/deployment.log\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════════╝\n")

	return sb.String()
}

func NewRunner(cfg *model.Config) (*Runner, error) {
	logger, err := utils.InitLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}
	return &Runner{
		Config: cfg,
		Logger: logger,
	}, nil
}

func (r *Runner) runCommand(name string, args []string, dir string, env []string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}

	// We capture combined output for logging
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Always log the output
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

		// Ensure logs directory exists
		os.MkdirAll("logs", 0755)

		// Write error to dedicated error file for easy access
		errorFile := filepath.Join("logs", fmt.Sprintf("%s_error.log", name))
		os.WriteFile(errorFile, output, 0644)

		// Return detailed error
		return &CommandError{
			Command: name,
			Args:    args,
			Output:  outputStr,
			Err:     err,
		}
	}

	return nil
}

func (r *Runner) Run() error {
	return r.RunWithBuildDir("build")
}

func (r *Runner) RunWithBuildDir(buildDir string) error {
	// Ensure build directory exists
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}

	fmt.Println("Generating OpenTofu configuration...")
	if err := generator.GenerateTofu(r.Config, buildDir); err != nil {
		return err
	}

	fmt.Println("Initializing OpenTofu...")
	if err := r.runCommand("tofu", []string{"init"}, buildDir, aws.EnvForStatic(r.Config.AWSAccessKey, r.Config.AWSSecretKey)); err != nil {
		return err
	}

	fmt.Println("Applying OpenTofu plan (this may take a while)...")
	if err := r.runCommand("tofu", []string{"apply", "-auto-approve"}, buildDir, aws.EnvForStatic(r.Config.AWSAccessKey, r.Config.AWSSecretKey)); err != nil {
		return err
	}

	// Fetch IPs and DNS names
	fmt.Println("Fetching instance information...")
	ips, err := r.GetTofuOutput(buildDir, "instance_ips")
	if err != nil {
		return fmt.Errorf("failed to get instance IPs: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no instances created")
	}

	dnsNames, err := r.GetTofuOutput(buildDir, "instance_dns_names")
	if err != nil {
		return fmt.Errorf("failed to get instance DNS names: %w", err)
	}
	if len(dnsNames) != len(ips) {
		return fmt.Errorf("mismatch between IPs and DNS names count")
	}

	fmt.Printf("Created %d instance(s):\n", len(ips))
	for i := range ips {
		fmt.Printf("  [%d] IP: %s, DNS: %s\n", i+1, ips[i], dnsNames[i])
	}

	// Generate Ansible Inventory/Site
	// For simplicity, we are generating site.yml and passing IPs via inventory flag or creating an inventory file.
	// The prompt asked to generate site.yml. We can create an inventory file 'hosts.ini' too.
	// But actually, we can pass comma separated IPs to ansible-playbook -i "ip,"

	// Create inventory file 'hosts.ini' with groups
	inventoryPath := filepath.Join(buildDir, "hosts.ini")
	invFile, err := os.Create(inventoryPath)
	if err != nil {
		return err
	}
	defer invFile.Close()

	// Helper to write host line
	writeHost := func(ip string, extraVars ...string) {
		line := fmt.Sprintf("%s ansible_user=ubuntu ansible_ssh_common_args='-o StrictHostKeyChecking=no'", ip)
		if r.Config.SSHPrivateKeyPath != "" {
			line += fmt.Sprintf(" ansible_ssh_private_key_file=%s", r.Config.SSHPrivateKeyPath)
		}
		for _, extra := range extraVars {
			line += " " + extra
		}
		invFile.WriteString(line + "\n")
	}

	invFile.WriteString("[init]\n")
	if len(ips) > 0 {
		// Add rancher_hostname variable for the init node
		writeHost(ips[0], fmt.Sprintf("rancher_hostname=%s", dnsNames[0]))
	}

	invFile.WriteString("\n[join]\n")
	if len(ips) > 1 {
		for _, ip := range ips[1:] {
			writeHost(ip)
		}
	}

	fmt.Println("Generating Ansible Playbook...")
	if err := generator.GenerateAnsible(r.Config, buildDir); err != nil {
		return err
	}

	// Wait for SSH to be available on all instances
	fmt.Println("Waiting for SSH to be available on all instances...")
	if err := r.waitForSSH(ips); err != nil {
		return fmt.Errorf("failed waiting for SSH: %w", err)
	}

	fmt.Println("Running Ansible Playbook...")
	if err := r.runCommand("ansible-playbook", []string{"-i", "hosts.ini", "site.yml"}, buildDir, nil); err != nil {
		return err
	}

	// Display success message with Rancher URL
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🎉 Deployment Complete!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\n📍 Rancher Dashboard: https://%s/dashboard\n", dnsNames[0])
	fmt.Printf("🔑 Username: admin\n")
	fmt.Printf("🔑 Password: admin\n")
	fmt.Printf("\n📋 Cluster Details:\n")
	fmt.Printf("   - Nodes: %d\n", len(ips))
	fmt.Printf("   - Region: %s\n", r.Config.AWSRegion)
	fmt.Printf("   - RKE2 Version: %s\n", r.Config.RKE2Version)
	fmt.Printf("   - Rancher Version: %s\n", r.Config.RancherVersion)
	fmt.Printf("\n📝 Full logs: logs/deployment.log\n")
	fmt.Println(strings.Repeat("=", 70) + "\n")

	return nil
}

func (r *Runner) GetTofuOutput(dir string, outputName string) ([]string, error) {
	cmd := exec.Command("tofu", "output", "-json", outputName)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var values []string
	if err := json.Unmarshal(output, &values); err != nil {
		return nil, err
	}
	return values, nil
}

// waitForSSH waits for SSH to be available on all instances
func (r *Runner) waitForSSH(ips []string) error {
	maxRetries := 30
	retryDelay := 10 // seconds

	for i, ip := range ips {
		fmt.Printf("  [%d/%d] Waiting for SSH on %s...\n", i+1, len(ips), ip)

		for retry := 0; retry < maxRetries; retry++ {
			// Try to connect with a simple SSH command
			cmd := exec.Command("ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				"-o", "BatchMode=yes",
				"-i", r.Config.SSHPrivateKeyPath,
				fmt.Sprintf("ubuntu@%s", ip),
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
				time.Sleep(time.Duration(retryDelay) * time.Second)
			} else {
				return fmt.Errorf("SSH not available on %s after %d attempts", ip, maxRetries)
			}
		}
	}

	fmt.Println("✓ All instances are ready for provisioning")
	return nil
}
