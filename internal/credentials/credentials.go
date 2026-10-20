package credentials

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const defaultCredentialsPath = "cloud-credentials.yaml"

// CloudCredentials holds all cloud provider credentials
type CloudCredentials struct {
	AWS []AWSCredential `yaml:"aws"`
	// Future: Azure, GCP, etc.
}

// AWSCredential represents AWS access credentials
type AWSCredential struct {
	Name          string `yaml:"name"`
	AccessKey     string `yaml:"access_key"`
	SecretKey     string `yaml:"secret_key"`
	DefaultRegion string `yaml:"default_region,omitempty"`
}

// LoadCredentials loads credentials from file
func LoadCredentials(path string) (*CloudCredentials, error) {
	// If file doesn't exist, return empty credentials
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &CloudCredentials{
			AWS: []AWSCredential{},
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds CloudCredentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	if creds.AWS == nil {
		creds.AWS = []AWSCredential{}
	}

	return &creds, nil
}

// Save writes credentials to file with secure permissions
func (c *CloudCredentials) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// File permission 0600 for security (only owner can read/write)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// AddAWSCredential adds or updates an AWS credential
func (c *CloudCredentials) AddAWSCredential(cred AWSCredential) error {
	// Check if credential with same name exists
	for i, existing := range c.AWS {
		if existing.Name == cred.Name {
			// Update existing
			c.AWS[i] = cred
			return nil
		}
	}

	// Add new credential
	c.AWS = append(c.AWS, cred)
	return nil
}

// DeleteAWSCredential removes an AWS credential by name
func (c *CloudCredentials) DeleteAWSCredential(name string) error {
	for i, cred := range c.AWS {
		if cred.Name == name {
			c.AWS = append(c.AWS[:i], c.AWS[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("credential '%s' not found", name)
}

// GetAWSCredential retrieves an AWS credential by name
func (c *CloudCredentials) GetAWSCredential(name string) (*AWSCredential, error) {
	for _, cred := range c.AWS {
		if cred.Name == name {
			return &cred, nil
		}
	}
	return nil, fmt.Errorf("credential '%s' not found", name)
}

// ListAWSCredentials returns all AWS credential names
func (c *CloudCredentials) ListAWSCredentials() []string {
	names := make([]string, len(c.AWS))
	for i, cred := range c.AWS {
		names[i] = cred.Name
	}
	return names
}

// HasAWSCredentials returns true if at least one AWS credential exists
func (c *CloudCredentials) HasAWSCredentials() bool {
	return len(c.AWS) > 0
}

// Validate checks if the credential is valid
func (a *AWSCredential) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("credential name is required")
	}
	if a.AccessKey == "" {
		return fmt.Errorf("AWS access key is required")
	}
	if a.SecretKey == "" {
		return fmt.Errorf("AWS secret key is required")
	}
	return nil
}
