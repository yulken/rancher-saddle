package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Felipalds/rancher-tui/internal/generator"
)

// UpgradeConfig holds all parameters needed to upgrade Rancher on a cluster.
type UpgradeConfig struct {
	ClusterName       string
	Distribution      string // "rke2" or "k3s"
	InitIP            string // first instance IP
	SSHPrivateKeyPath string
	SSHUser           string
	Hostname          string // DNS name or sslip.io address for --set hostname=
	RancherVersion    string
	BootstrapPassword string
	Prime             bool
	Replicas          int
	AuditLog          bool
	AuditLogLevel     int
	ImageTag          string
	Debug             bool
}

// templateData is the struct passed to the Go text/template renderer.
type templateData struct {
	InitIP            string
	SSHUser           string
	SSHPrivateKeyPath string
	Hostname          string
	RancherVersion    string
	BootstrapPassword string
	Prime             bool
	Replicas          int
	AuditLog          bool
	AuditLogLevel     int
	ImageTag          string
	Debug             bool
	Kubeconfig        string
	Kubectl           string
}

// Runner executes a Rancher upgrade via Ansible.
type Runner struct {
	Config   UpgradeConfig
	renderer *generator.TemplateRenderer
}

// NewRunner creates a new upgrade runner.
func NewRunner(cfg UpgradeConfig) *Runner {
	return &Runner{
		Config:   cfg,
		renderer: generator.NewTemplateRenderer(),
	}
}

func (r *Runner) templateData() templateData {
	kubeconfig := "/etc/rancher/rke2/rke2.yaml"
	kubectl := "/var/lib/rancher/rke2/bin/kubectl"
	if r.Config.Distribution == "k3s" {
		kubeconfig = "/etc/rancher/k3s/k3s.yaml"
		kubectl = "/usr/local/bin/k3s kubectl"
	}

	return templateData{
		InitIP:            r.Config.InitIP,
		SSHUser:           r.Config.SSHUser,
		SSHPrivateKeyPath: r.Config.SSHPrivateKeyPath,
		Hostname:          r.Config.Hostname,
		RancherVersion:    r.Config.RancherVersion,
		BootstrapPassword: r.Config.BootstrapPassword,
		Prime:             r.Config.Prime,
		Replicas:          r.Config.Replicas,
		AuditLog:          r.Config.AuditLog,
		AuditLogLevel:     r.Config.AuditLogLevel,
		ImageTag:          r.Config.ImageTag,
		Debug:             r.Config.Debug,
		Kubeconfig:        kubeconfig,
		Kubectl:           kubectl,
	}
}

// Run renders the Ansible templates and executes the upgrade playbook.
// logFile, if non-nil, receives all ansible-playbook stdout/stderr.
func (r *Runner) Run(logFile *os.File) error {
	workDir := filepath.Join("clusters", r.Config.ClusterName, "upgrade")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create upgrade work dir: %w", err)
	}

	data := r.templateData()
	templateDir := "internal/upgrade/templates"

	// Render inventory
	invTemplatePath := filepath.Join(templateDir, "upgrade-inventory.ini.tmpl")
	invOutputPath := filepath.Join(workDir, "hosts.ini")
	if err := r.renderer.Render(nil, invTemplatePath, data, invOutputPath); err != nil {
		return fmt.Errorf("failed to render inventory: %w", err)
	}

	// Render playbook
	pbTemplatePath := filepath.Join(templateDir, "upgrade-rancher.yml.tmpl")
	pbOutputPath := filepath.Join(workDir, "upgrade.yml")
	if err := r.renderer.Render(nil, pbTemplatePath, data, pbOutputPath); err != nil {
		return fmt.Errorf("failed to render playbook: %w", err)
	}

	// Run ansible-playbook
	cmd := exec.Command("ansible-playbook", "-i", "hosts.ini", "upgrade.yml")
	cmd.Dir = workDir
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ansible-playbook failed: %w", err)
	}

	return nil
}
