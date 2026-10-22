package views

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const importTestAWSConfig = `[profile acme-prod]
sso_session = acme-sso
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
region = us-east-1

[profile sandbox]
sso_start_url = https://sandbox.awsapps.com/start
sso_region = eu-central-1
sso_account_id = 111122223333
sso_role_name = ReadOnly

[profile plain-static]
region = eu-west-1

[sso-session acme-sso]
sso_start_url = https://example.awsapps.com/start
sso_region = us-west-2
`

func setupImportTest(t *testing.T) (configPath, storePath string) {
	t.Helper()
	dir := t.TempDir()

	configPath = filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(configPath, []byte(importTestAWSConfig), 0600))
	t.Setenv("AWS_CONFIG_FILE", configPath)

	return configPath, filepath.Join(dir, "cloud-credentials.yaml")
}

func TestLoadCredentialsWithDiscovery_ImportsOnFirstRun(t *testing.T) {
	_, storePath := setupImportTest(t)

	creds, imported, err := loadCredentialsWithDiscovery(storePath)
	require.NoError(t, err)
	assert.Equal(t, 2, imported)
	require.Len(t, creds.AWS, 2)

	byName := map[string]credentials.AWSCredential{}
	for _, cred := range creds.AWS {
		byName[cred.Name] = cred
	}

	acme, ok := byName["acme-prod"]
	require.True(t, ok)
	assert.Equal(t, core.AuthSSO, acme.AuthType())
	assert.Equal(t, "acme-prod", acme.Profile)
	assert.Equal(t, "us-west-2", acme.SSORegion)
	assert.Equal(t, "us-east-1", acme.DefaultRegion)
	assert.Empty(t, acme.AccessKey)
	assert.Empty(t, acme.SecretKey)

	// Persisted, so the next loader call sees a normal store.
	reloaded, err := credentials.LoadCredentials(storePath)
	require.NoError(t, err)
	assert.Len(t, reloaded.AWS, 2)
}

func TestLoadCredentialsWithDiscovery_DoesNotReimport(t *testing.T) {
	_, storePath := setupImportTest(t)

	// An existing store, even an empty one, means the user already had a chance
	// to remove profiles they do not want.
	require.NoError(t, os.WriteFile(storePath, []byte("aws: []\n"), 0600))

	_, imported, err := loadCredentialsWithDiscovery(storePath)
	require.NoError(t, err)
	assert.Zero(t, imported)
}

func TestImportSSOProfiles_SkipsNamesAndProfilesInUse(t *testing.T) {
	setupImportTest(t)

	creds := &credentials.CloudCredentials{AWS: []credentials.AWSCredential{
		{Name: "static-production", AccessKey: "AKIA", SecretKey: "secret"},
		{Name: "work", Type: core.AuthSSO, Profile: "acme-prod"},
	}}

	imported := importSSOProfiles(creds)

	assert.Equal(t, 1, imported, "only the sandbox profile is new")
	names := creds.ListAWSCredentials()
	assert.ElementsMatch(t, []string{"static-production", "work", "sandbox"}, names)
}

func TestImportSSOProfiles_NoProfiles(t *testing.T) {
	_, storePath := setupImportTest(t)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing"))

	creds, imported, err := loadCredentialsWithDiscovery(storePath)
	require.NoError(t, err)
	assert.Zero(t, imported)
	assert.Empty(t, creds.AWS)
}
