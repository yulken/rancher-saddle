package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/Felipalds/rancher-tui/cmd"
	"github.com/Felipalds/rancher-tui/internal/cluster"
	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/model"
	dockerorch "github.com/Felipalds/rancher-tui/internal/orchestrators/docker"
	"github.com/Felipalds/rancher-tui/internal/orchestrators/harvester"
	"github.com/Felipalds/rancher-tui/internal/orchestrators/k3s"
	"github.com/Felipalds/rancher-tui/internal/orchestrators/rke2"
	"github.com/Felipalds/rancher-tui/internal/providers/aws"
	"github.com/Felipalds/rancher-tui/internal/providers/libvirt"
	"github.com/spf13/cobra"
)

func init() {
	// Register providers
	core.GlobalRegistry.RegisterProvider(aws.NewProvider())
	core.GlobalRegistry.RegisterProvider(libvirt.NewProvider())

	// Register orchestrators
	core.GlobalRegistry.RegisterOrchestrator(rke2.NewOrchestrator())
	core.GlobalRegistry.RegisterOrchestrator(k3s.NewOrchestrator())
	core.GlobalRegistry.RegisterOrchestrator(dockerorch.NewOrchestrator())
	core.GlobalRegistry.RegisterOrchestrator(harvester.NewOrchestrator())
}

func main() {
	var configPath string
	var clusterName string
	var force bool

	var rootCmd = &cobra.Command{
		Use:   "rancher-tui",
		Short: "A TUI for creating and managing the Rancher product family",
		Long:  "rancher-tui is a terminal UI for creating and managing Rancher, Harvester, Longhorn, RKE2, K3s and related products across cloud and local environments",
		Run: func(c *cobra.Command, args []string) {
			// Launch the fullscreen TUI application
			cmd.LaunchTUI()
		},
	}

	// LIST command
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all clusters",
		Run: func(c *cobra.Command, args []string) {
			if err := cluster.ListClusters(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// CREATE command
	var createCmd = &cobra.Command{
		Use:   "create [cluster-name]",
		Short: "Create a new cluster",
		Args:  cobra.MaximumNArgs(1),
		Run: func(c *cobra.Command, args []string) {
			// Determine cluster name
			name := clusterName
			if len(args) > 0 {
				name = args[0]
			}

			// Load config for TUI
			tuiCfg, err := model.LoadConfig(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}

			// Launch TUI
			submitted, err := cmd.RunTUI(tuiCfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
				os.Exit(1)
			}

			if !submitted {
				fmt.Println("Deployment cancelled.")
				return
			}

			// Prompt for cluster name if not provided
			if name == "" {
				fmt.Print("Enter cluster name: ")
				fmt.Scanln(&name)
				if name == "" {
					fmt.Fprintf(os.Stderr, "Error: cluster name is required\n")
					os.Exit(1)
				}
			}

			// Convert to config format
			cfg := config.FromLegacyConfig(tuiCfg)

			// Validate
			if err := cfg.ValidateWithRegistry(core.GlobalRegistry); err != nil {
				fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
				os.Exit(1)
			}

			// Create cluster
			if err := cluster.CreateClusterNew(name, cfg, core.GlobalRegistry); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	createCmd.Flags().StringVarP(&clusterName, "name", "n", "", "Cluster name")

	// DELETE command
	var deleteCmd = &cobra.Command{
		Use:   "delete <cluster-name>",
		Short: "Delete a cluster",
		Args:  cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			name := args[0]
			if err := cluster.DeleteCluster(name, force); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	deleteCmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	// LIST-PROVIDERS command
	var listProvidersCmd = &cobra.Command{
		Use:   "list-providers",
		Short: "List all registered cloud providers",
		Run: func(c *cobra.Command, args []string) {
			providers := core.GlobalRegistry.ListProviders()
			fmt.Println("Registered Providers:")
			for _, p := range providers {
				fmt.Printf("  - %s\n", p)
			}
		},
	}

	// LIST-ORCHESTRATORS command
	var listOrchestratorsCmd = &cobra.Command{
		Use:   "list-orchestrators",
		Short: "List all registered Kubernetes orchestrators",
		Run: func(c *cobra.Command, args []string) {
			orchestrators := core.GlobalRegistry.ListOrchestrators()
			fmt.Println("Registered Orchestrators:")
			for _, o := range orchestrators {
				fmt.Printf("  - %s\n", o)
			}
		},
	}

	// SSO command group
	var ssoProfile string
	var ssoDeviceCode bool

	var ssoLoginCmd = &cobra.Command{
		Use:   "login",
		Short: "Authenticate an AWS SSO profile (delegates to `aws sso login`)",
		Run: func(c *cobra.Command, args []string) {
			cliArgs := []string{"sso", "login", "--profile", ssoProfile}
			if ssoDeviceCode {
				cliArgs = append(cliArgs, "--use-device-code")
			}
			cmd := exec.Command("aws", cliArgs...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✓ Logged in to %q\n", ssoProfile)
		},
	}
	ssoLoginCmd.Flags().StringVarP(&ssoProfile, "profile", "p", "", "AWS profile name")
	ssoLoginCmd.Flags().BoolVar(&ssoDeviceCode, "device-code", false, "Use the device authorization flow (no local browser)")
	_ = ssoLoginCmd.MarkFlagRequired("profile")

	var ssoStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show the login status of AWS SSO profiles",
		Run: func(c *cobra.Command, args []string) {
			profiles, err := aws.ListSSOProfiles()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if ssoProfile != "" {
				profiles = filterProfiles(profiles, ssoProfile)
				if len(profiles) == 0 {
					fmt.Fprintf(os.Stderr, "Profile %q is not configured for SSO in %s\n", ssoProfile, aws.DefaultAWSConfigPath())
					os.Exit(1)
				}
			}
			if len(profiles) == 0 {
				fmt.Printf("No SSO profiles found in %s\n", aws.DefaultAWSConfigPath())
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			for _, p := range profiles {
				fmt.Printf("%-24s %s\n", p.Name, aws.DescribeSession(ctx, p.Name))
			}
		},
	}
	ssoStatusCmd.Flags().StringVarP(&ssoProfile, "profile", "p", "", "Limit output to one profile")

	var ssoCmd = &cobra.Command{
		Use:   "sso",
		Short: "AWS IAM Identity Center helpers",
	}
	ssoCmd.AddCommand(ssoLoginCmd, ssoStatusCmd)

	// Add commands to root
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listProvidersCmd)
	rootCmd.AddCommand(listOrchestratorsCmd)
	rootCmd.AddCommand(ssoCmd)

	// Flags for root command (shared)
	rootCmd.Flags().StringVarP(&clusterName, "name", "n", "", "Cluster name")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "config.yaml", "Path to configuration file")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// filterProfiles keeps only the named profile.
func filterProfiles(profiles []aws.SSOProfile, name string) []aws.SSOProfile {
	out := make([]aws.SSOProfile, 0, 1)
	for _, p := range profiles {
		if p.Name == name {
			out = append(out, p)
		}
	}
	return out
}
