package shared

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// SharedDir returns the path to the shared orchestrator directory
func SharedDir() string {
	return "internal/orchestrators/shared"
}

// ReadAddonsTemplate reads the shared addons template
func ReadAddonsTemplate() ([]byte, error) {
	return os.ReadFile(filepath.Join(SharedDir(), "templates", "addons.yml.tmpl"))
}

// IndentYAML indents YAML content by the specified number of spaces
func IndentYAML(content string, spaces int) string {
	indent := ""
	for i := 0; i < spaces; i++ {
		indent += " "
	}

	result := ""
	lines := SplitLines(content)
	for _, line := range lines {
		if line != "" {
			result += indent + line + "\n"
		} else {
			result += "\n"
		}
	}

	return result
}

// SplitLines splits a string by newline characters
func SplitLines(s string) []string {
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

// GenerateInventory generates the Ansible inventory file for any orchestrator
func GenerateInventory(outputs *core.InfrastructureOutputs, config map[string]interface{}, outputDir string) error {
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

	invFile.WriteString("[init]\n")
	if len(outputs.InstanceIPs) > 0 {
		rancherHostname := ""
		if len(outputs.InstanceDNSNames) > 0 {
			rancherHostname = outputs.InstanceDNSNames[0]
		} else {
			rancherHostname = outputs.InstanceIPs[0]
		}
		writeHost(outputs.InstanceIPs[0], fmt.Sprintf("rancher_hostname=%s", rancherHostname))
	}

	invFile.WriteString("\n[join]\n")
	if len(outputs.InstanceIPs) > 1 {
		for _, ip := range outputs.InstanceIPs[1:] {
			writeHost(ip)
		}
	}

	return nil
}
