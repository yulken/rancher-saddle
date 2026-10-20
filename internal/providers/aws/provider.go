package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/generator"
)

// Provider implements the AWS cloud provider
type Provider struct {
	renderer *generator.TemplateRenderer
}

// NewProvider creates a new AWS provider instance
func NewProvider() *Provider {
	return &Provider{
		renderer: generator.NewTemplateRenderer(),
	}
}

// Name returns the provider type
func (p *Provider) Name() core.ProviderType {
	return core.ProviderAWS
}

// Validate validates AWS-specific configuration
func (p *Provider) Validate(config map[string]interface{}) error {
	return Validate(config)
}

// GenerateInfrastructure generates Terraform configuration for AWS
func (p *Provider) GenerateInfrastructure(ctx context.Context, config map[string]interface{}, outputDir string) error {
	// Parse AWS config
	awsConfig := &AWSConfig{}
	awsConfig.FromMap(config)

	// Get template path (relative to this package)
	templatePath := filepath.Join(getPackageDir(), "templates", "main.tf.tmpl")

	// Output path
	outputPath := filepath.Join(outputDir, "main.tf")

	// Render template
	return p.renderer.Render(ctx, templatePath, awsConfig, outputPath)
}

// GetOutputs retrieves infrastructure outputs from Terraform
func (p *Provider) GetOutputs(ctx context.Context, buildDir string) (*core.InfrastructureOutputs, error) {
	// Get instance IPs
	ips, err := getTofuOutput(buildDir, "instance_ips")
	if err != nil {
		return nil, fmt.Errorf("failed to get instance IPs: %w", err)
	}

	// Get DNS names
	dnsNames, err := getTofuOutput(buildDir, "instance_dns_names")
	if err != nil {
		return nil, fmt.Errorf("failed to get instance DNS names: %w", err)
	}

	return &core.InfrastructureOutputs{
		InstanceIPs:       ips,
		InstanceDNSNames:  dnsNames,
		PrivateIPs:        []string{}, // AWS EC2 instances can have private IPs, but we're using public IPs here
		AdditionalOutputs: make(map[string]interface{}),
	}, nil
}

// GetRequiredFields returns the configuration fields required by AWS
func (p *Provider) GetRequiredFields() []core.FormField {
	return GetRequiredFields()
}

// GetDefaultConfig returns default configuration for AWS
func (p *Provider) GetDefaultConfig() map[string]interface{} {
	return GetDefaultConfig()
}

// Helper function to get Terraform output
func getTofuOutput(dir string, outputName string) ([]string, error) {
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

// Helper function to get the package directory
func getPackageDir() string {
	// This is a bit of a hack, but works for now
	// In production, you might want to use embed.FS or a more robust solution
	return "internal/providers/aws"
}
