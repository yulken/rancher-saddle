package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAWSConfig_FromMap_AllFields(t *testing.T) {
	m := map[string]interface{}{
		"access_key":        "AKIA123",
		"secret_key":        "secret",
		"region":            "eu-west-1",
		"subnet_id":         "subnet-abc",
		"security_group_id": "sg-def",
		"ami":               "ami-custom",
		"instance_type":     "m5.2xlarge",
		"root_volume_size":  50,
		"ssh_key_name":      "my-key",
		"node_prefix":       "prod-node",
		"instance_count":    5,
	}

	cfg := &AWSConfig{}
	cfg.FromMap(m)

	assert.Equal(t, "AKIA123", cfg.AccessKey)
	assert.Equal(t, "secret", cfg.SecretKey)
	assert.Equal(t, "eu-west-1", cfg.Region)
	assert.Equal(t, "subnet-abc", cfg.SubnetID)
	assert.Equal(t, "sg-def", cfg.SecurityGroupID)
	assert.Equal(t, "ami-custom", cfg.AMI)
	assert.Equal(t, "m5.2xlarge", cfg.InstanceType)
	assert.Equal(t, 50, cfg.RootVolumeSize)
	assert.Equal(t, "my-key", cfg.SSHKeyName)
	assert.Equal(t, "prod-node", cfg.NodePrefix)
	assert.Equal(t, 5, cfg.InstanceCount)
}

func TestAWSConfig_FromMap_Defaults(t *testing.T) {
	cfg := &AWSConfig{}
	cfg.FromMap(map[string]interface{}{})

	assert.Equal(t, "us-east-1", cfg.Region)
	assert.Equal(t, "t3.xlarge", cfg.InstanceType)
	assert.Equal(t, 20, cfg.RootVolumeSize)
	assert.Equal(t, "ami-0c58b2975bef51185", cfg.AMI)
	assert.Equal(t, "k8s-node", cfg.NodePrefix)
	assert.Equal(t, 1, cfg.InstanceCount)
}

func TestAWSConfig_FromMap_Float64Conversion(t *testing.T) {
	// YAML/JSON unmarshaling often produces float64 for numbers
	m := map[string]interface{}{
		"root_volume_size": float64(100),
		"instance_count":   float64(3),
	}

	cfg := &AWSConfig{}
	cfg.FromMap(m)

	assert.Equal(t, 100, cfg.RootVolumeSize)
	assert.Equal(t, 3, cfg.InstanceCount)
}

func TestAWSConfig_FromMap_IntConversion(t *testing.T) {
	m := map[string]interface{}{
		"root_volume_size": 80,
		"instance_count":   7,
	}

	cfg := &AWSConfig{}
	cfg.FromMap(m)

	assert.Equal(t, 80, cfg.RootVolumeSize)
	assert.Equal(t, 7, cfg.InstanceCount)
}

func TestGetRequiredFields(t *testing.T) {
	fields := GetRequiredFields()

	assert.True(t, len(fields) > 0)

	// Check key required fields exist
	fieldNames := make(map[string]bool)
	for _, f := range fields {
		fieldNames[f.Name] = true
	}
	assert.True(t, fieldNames["access_key"])
	assert.True(t, fieldNames["secret_key"])
	assert.True(t, fieldNames["region"])
	assert.True(t, fieldNames["subnet_id"])
	assert.True(t, fieldNames["security_group_id"])
}

func TestGetDefaultConfig(t *testing.T) {
	defaults := GetDefaultConfig()

	assert.Equal(t, "us-east-1", defaults["region"])
	assert.Equal(t, "t3.xlarge", defaults["instance_type"])
	assert.Equal(t, 20, defaults["root_volume_size"])
}

func TestAWSConfig_FromMap_SSO(t *testing.T) {
	cfg := &AWSConfig{}
	cfg.FromMap(map[string]interface{}{
		"auth":              "sso",
		"profile":           "acme-prod",
		"region":            "eu-west-1",
		"subnet_id":         "subnet-abc",
		"security_group_id": "sg-def",
		// Keys from a legacy entry must not leak into SSO mode's decisions.
		"access_key": "AKIA-STALE",
		"secret_key": "stale",
	})

	assert.Equal(t, "sso", cfg.AuthMode)
	assert.Equal(t, "acme-prod", cfg.Profile)
	assert.Equal(t, "eu-west-1", cfg.Region)
}

func TestAWSConfig_FromMap_DefaultsToStaticAuth(t *testing.T) {
	cfg := &AWSConfig{}
	cfg.FromMap(map[string]interface{}{
		"access_key":        "AKIA123",
		"secret_key":        "secret",
		"region":            "us-east-1",
		"subnet_id":         "subnet-abc",
		"security_group_id": "sg-def",
	})

	assert.Equal(t, "static", cfg.AuthMode)
	assert.Empty(t, cfg.Profile)
}
