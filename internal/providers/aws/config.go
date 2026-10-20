package aws

import (
	"github.com/Felipalds/rancher-tui/internal/core"
)

// AWSConfig holds AWS-specific configuration
type AWSConfig struct {
	AccessKey       string
	SecretKey       string
	Region          string
	SubnetID        string
	SecurityGroupID string
	AMI             string
	InstanceType    string
	RootVolumeSize  int
	SSHKeyName      string
	NodePrefix      string
	InstanceCount   int
}

// FromMap creates AWSConfig from a map
func (c *AWSConfig) FromMap(m map[string]interface{}) {
	if v, ok := m["access_key"].(string); ok {
		c.AccessKey = v
	}
	if v, ok := m["secret_key"].(string); ok {
		c.SecretKey = v
	}
	if v, ok := m["region"].(string); ok {
		c.Region = v
	}
	if v, ok := m["subnet_id"].(string); ok {
		c.SubnetID = v
	}
	if v, ok := m["security_group_id"].(string); ok {
		c.SecurityGroupID = v
	}
	if v, ok := m["ami"].(string); ok {
		c.AMI = v
	}
	if v, ok := m["instance_type"].(string); ok {
		c.InstanceType = v
	}
	if v, ok := m["root_volume_size"].(int); ok {
		c.RootVolumeSize = v
	} else if v, ok := m["root_volume_size"].(float64); ok {
		c.RootVolumeSize = int(v)
	}
	if v, ok := m["ssh_key_name"].(string); ok {
		c.SSHKeyName = v
	}
	if v, ok := m["node_prefix"].(string); ok {
		c.NodePrefix = v
	}
	if v, ok := m["instance_count"].(int); ok {
		c.InstanceCount = v
	} else if v, ok := m["instance_count"].(float64); ok {
		c.InstanceCount = int(v)
	}

	// Apply defaults
	if c.Region == "" {
		c.Region = "us-east-1"
	}
	if c.InstanceType == "" {
		c.InstanceType = "t3.xlarge"
	}
	if c.RootVolumeSize == 0 {
		c.RootVolumeSize = 20
	}
	if c.AMI == "" {
		// Default to Ubuntu 22.04 LTS in us-west-2
		c.AMI = "ami-0c58b2975bef51185"
	}
	if c.NodePrefix == "" {
		c.NodePrefix = "k8s-node"
	}
	if c.InstanceCount == 0 {
		c.InstanceCount = 1
	}
}

// GetRequiredFields returns the form fields for AWS configuration
func GetRequiredFields() []core.FormField {
	return []core.FormField{
		{
			Name:        "access_key",
			Label:       "AWS Access Key",
			Description: "AWS IAM access key ID",
			Required:    true,
			Type:        "string",
		},
		{
			Name:        "secret_key",
			Label:       "AWS Secret Key",
			Description: "AWS IAM secret access key",
			Required:    true,
			Type:        "string",
		},
		{
			Name:        "region",
			Label:       "AWS Region",
			Description: "AWS region to deploy resources",
			Required:    true,
			Default:     "us-east-1",
			Type:        "string",
		},
		{
			Name:        "subnet_id",
			Label:       "Subnet ID",
			Description: "VPC subnet ID for instances",
			Required:    true,
			Type:        "string",
		},
		{
			Name:        "security_group_id",
			Label:       "Security Group ID",
			Description: "Security group ID for instances",
			Required:    true,
			Type:        "string",
		},
		{
			Name:        "ami",
			Label:       "AMI ID",
			Description: "Amazon Machine Image ID (defaults to Ubuntu 22.04)",
			Required:    false,
			Default:     "ami-0c58b2975bef51185",
			Type:        "string",
		},
		{
			Name:        "instance_type",
			Label:       "Instance Type",
			Description: "EC2 instance type",
			Required:    false,
			Default:     "t3.xlarge",
			Type:        "string",
		},
		{
			Name:        "root_volume_size",
			Label:       "Root Volume Size (GB)",
			Description: "Size of the root EBS volume in GB",
			Required:    false,
			Default:     20,
			Type:        "int",
		},
	}
}

// GetDefaultConfig returns default AWS configuration
func GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"region":           "us-east-1",
		"ami":              "ami-0c58b2975bef51185",
		"instance_type":    "t3.xlarge",
		"root_volume_size": 20,
	}
}
