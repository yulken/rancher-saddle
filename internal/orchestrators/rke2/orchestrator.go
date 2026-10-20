package rke2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/generator"
)

// Orchestrator implements the RKE2 Kubernetes orchestrator
type Orchestrator struct {
	renderer *generator.TemplateRenderer
}

// NewOrchestrator creates a new RKE2 orchestrator instance
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		renderer: generator.NewTemplateRenderer(),
	}
}

// Name returns the orchestrator type
func (o *Orchestrator) Name() core.OrchestratorType {
	return core.OrchestratorRKE2
}

// Validate validates RKE2-specific configuration
func (o *Orchestrator) Validate(config map[string]interface{}) error {
	// RKE2 version is optional, has a default
	// Rancher version is optional, has a default
	// No strict validation needed for now
	return nil
}

// GeneratePlaybook generates the Ansible playbook for RKE2 deployment
func (o *Orchestrator) GeneratePlaybook(ctx context.Context, config map[string]interface{}, outputDir string) error {
	// Parse RKE2 config
	rke2Config := &RKE2Config{}
	rke2Config.FromMap(config)

	// Read module templates
	initTasksContent, err := os.ReadFile(filepath.Join(getPackageDir(), "templates", "init.yml.tmpl"))
	if err != nil {
		return fmt.Errorf("failed to read init tasks template: %w", err)
	}

	joinTasksContent, err := os.ReadFile(filepath.Join(getPackageDir(), "templates", "join.yml.tmpl"))
	if err != nil {
		return fmt.Errorf("failed to read join tasks template: %w", err)
	}

	addonTasksContent, err := os.ReadFile(filepath.Join(getPackageDir(), "templates", "addons.yml.tmpl"))
	if err != nil {
		return fmt.Errorf("failed to read addons tasks template: %w", err)
	}

	// Populate task content in config
	rke2Config.InitTasks = indentYAML(string(initTasksContent), 4)
	rke2Config.JoinTasks = indentYAML(string(joinTasksContent), 4)
	rke2Config.AddonTasks = indentYAML(string(addonTasksContent), 4)

	// Get playbook template path
	templatePath := filepath.Join(getPackageDir(), "templates", "playbook.yml.tmpl")

	// Output path
	outputPath := filepath.Join(outputDir, "site.yml")

	// Render playbook
	return o.renderer.Render(ctx, templatePath, rke2Config, outputPath)
}

// GenerateInventory generates the Ansible inventory file
func (o *Orchestrator) GenerateInventory(ctx context.Context, outputs *core.InfrastructureOutputs, config map[string]interface{}, outputDir string) error {
	inventoryPath := filepath.Join(outputDir, "hosts.ini")
	invFile, err := os.Create(inventoryPath)
	if err != nil {
		return fmt.Errorf("failed to create inventory file: %w", err)
	}
	defer invFile.Close()

	// Get SSH settings from config
	sshKeyPath := ""
	sshUser := "ubuntu"

	if v, ok := config["ssh_private_key_path"].(string); ok {
		sshKeyPath = v
	}
	if v, ok := config["ssh_user"].(string); ok {
		sshUser = v
	}

	// Helper to write host line
	writeHost := func(ip string, extraVars ...string) {
		line := fmt.Sprintf("%s ansible_user=%s ansible_ssh_common_args='-o StrictHostKeyChecking=no'", ip, sshUser)
		if sshKeyPath != "" {
			line += fmt.Sprintf(" ansible_ssh_private_key_file=%s", sshKeyPath)
		}
		for _, extra := range extraVars {
			line += " " + extra
		}
		invFile.WriteString(line + "\n")
	}

	// Write init group (first node)
	invFile.WriteString("[init]\n")
	if len(outputs.InstanceIPs) > 0 {
		// Add rancher_hostname variable for the init node
		rancherHostname := ""
		if len(outputs.InstanceDNSNames) > 0 {
			rancherHostname = outputs.InstanceDNSNames[0]
		} else {
			rancherHostname = outputs.InstanceIPs[0]
		}
		writeHost(outputs.InstanceIPs[0], fmt.Sprintf("rancher_hostname=%s", rancherHostname))
	}

	// Write join group (remaining nodes)
	invFile.WriteString("\n[join]\n")
	if len(outputs.InstanceIPs) > 1 {
		for _, ip := range outputs.InstanceIPs[1:] {
			writeHost(ip)
		}
	}

	return nil
}

// GetRequiredFields returns the configuration fields required by RKE2
func (o *Orchestrator) GetRequiredFields() []core.FormField {
	return GetRequiredFields()
}

// GetDefaultConfig returns default configuration for RKE2
func (o *Orchestrator) GetDefaultConfig() map[string]interface{} {
	return GetDefaultConfig()
}

// GetModules returns the logical deployment modules for RKE2
func (o *Orchestrator) GetModules() []core.Module {
	// For now, we don't expose modules externally
	// They are used internally for template composition
	return []core.Module{}
}

// Helper function to get the package directory
func getPackageDir() string {
	return "internal/orchestrators/rke2"
}

// Helper function to indent YAML content
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

// Helper function to split string by newlines
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
