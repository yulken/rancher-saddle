package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/providers/aws"
	"github.com/Felipalds/rancher-tui/internal/providers/libvirt"
	"github.com/Felipalds/rancher-tui/internal/workflow"
)

const defaultConfigPath = "config.yaml"

// ANSI color codes for status indicators
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// getStatusDisplay returns a color-coded status indicator
func getStatusDisplay(status string) string {
	switch status {
	case "running":
		return colorGreen + "● running" + colorReset
	case "pending":
		return colorYellow + "⚠ pending" + colorReset
	case "failed":
		return colorRed + "✗ failed" + colorReset
	case "creating":
		return colorCyan + "⟳ creating" + colorReset
	case "deleting":
		return colorGray + "◐ deleting" + colorReset
	default:
		return colorGray + "○ " + status + colorReset
	}
}

// ListClusters displays all clusters in a table format
func ListClusters() error {
	cfg, err := config.LoadClustersConfig(defaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Clusters) == 0 {
		fmt.Println("No clusters found.")
		fmt.Println("\nUse 'rancher-tui create' to create a new cluster.")
		return nil
	}

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tNODES\tREGION\tCREATED\tRANCHER URL")
	fmt.Fprintln(w, strings.Repeat("-", 80))

	for name, cluster := range cfg.Clusters {
		age := formatAge(cluster.CreatedAt)
		nodeCount := cluster.Cluster.InstanceCount

		// Get region from provider config
		region := "-"
		if r, ok := cluster.Provider.Config["region"].(string); ok {
			region = r
		}

		rancherURL := cluster.RancherURL
		if rancherURL == "" {
			rancherURL = "-"
		}

		status := cluster.Status
		if status == "" {
			status = "unknown"
		}

		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
			name,
			getStatusDisplay(status),
			nodeCount,
			region,
			age,
			rancherURL,
		)
	}

	w.Flush()
	return nil
}

// CreateClusterNew creates a new cluster using the modular architecture
func CreateClusterNew(name string, cfg *config.Config, registry *core.Registry) error {
	// Load clusters config file
	clustersCfg, err := config.LoadClustersConfig(defaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if cluster already exists
	if _, exists := clustersCfg.GetCluster(name); exists {
		return fmt.Errorf("cluster '%s' already exists", name)
	}

	// Create cluster config entry
	clusterCfg := config.FromModernConfig(cfg)
	clusterCfg.Status = "creating"
	clusterCfg.BuildDir = filepath.Join("clusters", name)

	// Add to config
	clustersCfg.AddCluster(name, clusterCfg)

	// Save config
	if err := clustersCfg.Save(defaultConfigPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Creating cluster '%s'...\n", name)

	// Create build directory
	buildDir := filepath.Join("clusters", name)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Run deployment workflow
	runner, err := workflow.NewModularRunner(cfg, registry)
	if err != nil {
		return fmt.Errorf("failed to create workflow runner: %w", err)
	}
	if err := runner.RunWithBuildDir(buildDir); err != nil {
		// Update status to failed
		clusterCfg.Status = "failed"
		clustersCfg.AddCluster(name, clusterCfg)
		clustersCfg.Save(defaultConfigPath)
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Get infrastructure outputs
	provider, err := registry.GetProvider(cfg.GetProviderType())
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	outputs, err := provider.GetOutputs(nil, buildDir)
	if err != nil {
		fmt.Printf("Warning: failed to get infrastructure outputs: %v\n", err)
	} else {
		clusterCfg.InstanceIPs = outputs.InstanceIPs
		clusterCfg.InstanceDNS = outputs.InstanceDNSNames

		// Set Rancher URL
		if len(outputs.InstanceDNSNames) > 0 {
			clusterCfg.RancherURL = fmt.Sprintf("https://%s/dashboard", outputs.InstanceDNSNames[0])
		} else if len(outputs.InstanceIPs) > 0 {
			clusterCfg.RancherURL = fmt.Sprintf("https://%s/dashboard", outputs.InstanceIPs[0])
		}
	}

	// Update status to running
	clusterCfg.Status = "running"
	clustersCfg.AddCluster(name, clusterCfg)

	// Save final config
	if err := clustersCfg.Save(defaultConfigPath); err != nil {
		return fmt.Errorf("failed to save final config: %w", err)
	}

	fmt.Printf("\n✓ Cluster '%s' created successfully!\n", name)
	if clusterCfg.RancherURL != "" {
		fmt.Printf("Rancher URL: %s\n", clusterCfg.RancherURL)
	}

	return nil
}

// DeleteCluster deletes a cluster and its resources
func DeleteCluster(name string, force bool) error {
	// Load clusters config
	cfg, err := config.LoadClustersConfig(defaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if cluster exists
	cluster, exists := cfg.GetCluster(name)
	if !exists {
		return fmt.Errorf("cluster '%s' not found", name)
	}

	// Confirm deletion
	if !force {
		fmt.Printf("Are you sure you want to delete cluster '%s'? (yes/no): ", name)
		var response string
		fmt.Scanln(&response)
		if response != "yes" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	fmt.Printf("Deleting cluster '%s'...\n", name)

	// Preflight before touching any state: an expired SSO session would
	// otherwise mark the cluster as "deleting" and then fail silently inside
	// tofu destroy, which is the worst possible outcome for the user.
	if cluster.Provider.Type == string(core.ProviderAWS) {
		if err := aws.CheckSession(context.Background(), cluster.Provider.Config); err != nil {
			return fmt.Errorf("cannot destroy cluster '%s': %w\n"+
				"Run `aws sso login --profile %s` and try again", name, err, aws.Profile(cluster.Provider.Config))
		}
	}

	// Update status to deleting
	cluster.Status = "deleting"
	cfg.AddCluster(name, cluster)
	cfg.Save(defaultConfigPath)

	// Destroy infrastructure
	buildDir := cluster.BuildDir
	if buildDir == "" {
		buildDir = filepath.Join("clusters", name)
	}

	if cluster.Provider.Type == string(core.ProviderLibvirt) || cluster.Provider.Type == "vagrant" {
		fmt.Println("Tearing down libvirt VMs, network and disks...")
		if err := libvirt.Teardown(context.Background(), name, buildDir, os.Stdout); err != nil {
			fmt.Printf("Warning: libvirt teardown reported: %v\n", err)
		}
		if err := os.RemoveAll(buildDir); err != nil {
			fmt.Printf("Warning: failed to remove build directory: %v\n", err)
		}
	} else if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		fmt.Println("Destroying infrastructure...")
		cmd := exec.Command("tofu", "destroy", "-auto-approve")
		cmd.Dir = buildDir
		if cluster.Provider.Type == string(core.ProviderAWS) {
			cmd.Env = aws.EnvFor(cluster.Provider.Config)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: failed to destroy infrastructure: %v\n", err)
			fmt.Println("You may need to manually clean up AWS resources.")
		}

		// Remove build directory
		fmt.Println("Removing build directory...")
		if err := os.RemoveAll(buildDir); err != nil {
			fmt.Printf("Warning: failed to remove build directory: %v\n", err)
		}
	}

	// Remove from config
	cfg.DeleteCluster(name)

	// Save config
	if err := cfg.Save(defaultConfigPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Cluster '%s' deleted successfully!\n", name)
	return nil
}

// formatAge converts a time to a human-readable age string
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	duration := time.Since(t)
	hours := int(duration.Hours())

	if hours < 1 {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%dm", minutes)
	} else if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	} else {
		days := hours / 24
		return fmt.Sprintf("%dd", days)
	}
}
