package aws

import (
	"fmt"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// Validate validates AWS-specific configuration. Credential requirements depend
// on the authentication mode: static auth needs a key pair, SSO needs a profile.
func Validate(config map[string]interface{}) error {
	requiredFields := []string{
		"region",
		"subnet_id",
		"security_group_id",
	}

	switch AuthMode(config) {
	case core.AuthSSO:
		if Profile(config) == "" {
			return fmt.Errorf("AWS config: 'profile' is required when auth is '%s'", core.AuthSSO)
		}
	default:
		requiredFields = append(requiredFields, "access_key", "secret_key")
	}

	for _, field := range requiredFields {
		if v, ok := config[field].(string); !ok || v == "" {
			return fmt.Errorf("AWS config: '%s' is required", field)
		}
	}

	// Validate instance count if present
	if count, ok := config["instance_count"].(float64); ok {
		if count < 1 {
			return fmt.Errorf("AWS config: instance_count must be at least 1")
		}
	}

	// Validate root volume size if present
	if size, ok := config["root_volume_size"].(float64); ok {
		if size < 8 {
			return fmt.Errorf("AWS config: root_volume_size must be at least 8 GB")
		}
	}

	return nil
}
