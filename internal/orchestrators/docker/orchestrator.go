package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/generator"
)

// Orchestrator implements the Docker Rancher orchestrator.
// It installs Docker on the target node and runs Rancher as a container.
type Orchestrator struct {
	renderer *generator.TemplateRenderer
}

// NewOrchestrator creates a new Docker Rancher orchestrator instance
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		renderer: generator.NewTemplateRenderer(),
	}
}

// Name returns the orchestrator type
func (o *Orchestrator) Name() core.OrchestratorType {
	return core.OrchestratorDocker
}

// Validate validates Docker Rancher configuration
func (o *Orchestrator) Validate(config map[string]interface{}) error {
	return nil
}

// GeneratePlaybook generates the Ansible playbook for Docker Rancher deployment
func (o *Orchestrator) GeneratePlaybook(ctx context.Context, config map[string]interface{}, outputDir string) error {
	cfg := &DockerRancherConfig{}
	cfg.FromMap(config)

	// Read install tasks template
	installContent, err := os.ReadFile(filepath.Join(getPackageDir(), "templates", "install.yml.tmpl"))
	if err != nil {
		return fmt.Errorf("failed to read install tasks template: %w", err)
	}

	cfg.InstallTasks = indentYAML(string(installContent), 4)

	// Render playbook
	templatePath := filepath.Join(getPackageDir(), "templates", "playbook.yml.tmpl")
	outputPath := filepath.Join(outputDir, "site.yml")

	return o.renderer.Render(ctx, templatePath, cfg, outputPath)
}

// GenerateInventory generates the Ansible inventory file (single node, no join group)
func (o *Orchestrator) GenerateInventory(ctx context.Context, outputs *core.InfrastructureOutputs, config map[string]interface{}, outputDir string) error {
	inventoryPath := filepath.Join(outputDir, "hosts.ini")
	invFile, err := os.Create(inventoryPath)
	if err != nil {
		return fmt.Errorf("failed to create inventory file: %w", err)
	}
	defer invFile.Close()

	sshKeyPath := ""
	sshUser := "ubuntu"

	if v, ok := config["ssh_private_key_path"].(string); ok {
		sshKeyPath = v
	}
	if v, ok := config["ssh_user"].(string); ok {
		sshUser = v
	}

	// Write init group (single node)
	invFile.WriteString("[init]\n")
	if len(outputs.InstanceIPs) > 0 {
		rancherHostname := ""
		if len(outputs.InstanceDNSNames) > 0 {
			rancherHostname = outputs.InstanceDNSNames[0]
		} else {
			rancherHostname = outputs.InstanceIPs[0]
		}

		line := fmt.Sprintf("%s ansible_user=%s ansible_ssh_common_args='-o StrictHostKeyChecking=no'",
			outputs.InstanceIPs[0], sshUser)
		if sshKeyPath != "" {
			line += fmt.Sprintf(" ansible_ssh_private_key_file=%s", sshKeyPath)
		}
		line += fmt.Sprintf(" rancher_hostname=%s", rancherHostname)
		invFile.WriteString(line + "\n")
	}

	// No [join] group — Docker Rancher is single-node
	return nil
}

// GetRequiredFields returns the configuration fields required by Docker Rancher
func (o *Orchestrator) GetRequiredFields() []core.FormField {
	return GetRequiredFields()
}

// GetDefaultConfig returns default configuration for Docker Rancher
func (o *Orchestrator) GetDefaultConfig() map[string]interface{} {
	return GetDefaultConfig()
}

// GetModules returns empty — Docker Rancher has no modules
func (o *Orchestrator) GetModules() []core.Module {
	return []core.Module{}
}

func getPackageDir() string {
	return "internal/orchestrators/docker"
}

func indentYAML(content string, spaces int) string {
	indent := ""
	for i := 0; i < spaces; i++ {
		indent += " "
	}

	result := ""
	lines := splitLines(content)
	for _, line := range lines {
		if line != "" {
			result += indent + line + "\n"
		} else {
			result += "\n"
		}
	}

	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
