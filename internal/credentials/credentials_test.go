package credentials

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAWSCredential_AuthType(t *testing.T) {
	tests := []struct {
		name string
		cred AWSCredential
		want string
	}{
		{name: "legacy empty type is static", cred: AWSCredential{}, want: AuthStatic},
		{name: "explicit static", cred: AWSCredential{Type: "static"}, want: AuthStatic},
		{name: "sso", cred: AWSCredential{Type: "sso"}, want: AuthSSO},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cred.AuthType())
		})
	}
}

func TestAWSCredential_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cred    AWSCredential
		wantErr string
	}{
		{
			name: "static with keys",
			cred: AWSCredential{Name: "static", AccessKey: "AKIA", SecretKey: "secret"},
		},
		{
			name:    "name required",
			cred:    AWSCredential{AccessKey: "AKIA", SecretKey: "secret"},
			wantErr: "credential name is required",
		},
		{
			name:    "static missing access key",
			cred:    AWSCredential{Name: "x", SecretKey: "secret"},
			wantErr: "AWS access key is required",
		},
		{
			name:    "static missing secret key",
			cred:    AWSCredential{Name: "x", AccessKey: "AKIA"},
			wantErr: "AWS secret key is required",
		},
		{
			name: "sso with profile",
			cred: AWSCredential{Name: "sso", Type: AuthSSO, Profile: "acme-prod"},
		},
		{
			name:    "sso missing profile",
			cred:    AWSCredential{Name: "sso", Type: AuthSSO},
			wantErr: "SSO profile is required",
		},
		{
			name:    "unknown type",
			cred:    AWSCredential{Name: "x", Type: "carrier-pigeon"},
			wantErr: "unknown credential type 'carrier-pigeon'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cred.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoadCredentials_LegacyFileHasNoType(t *testing.T) {
	path := t.TempDir() + "/cloud-credentials.yaml"
	legacy := "aws:\n  - name: old\n    access_key: AKIA123\n    secret_key: shhh\n    default_region: us-east-1\n"

	require.NoError(t, os.WriteFile(path, []byte(legacy), 0600))

	creds, err := LoadCredentials(path)
	require.NoError(t, err)
	require.Len(t, creds.AWS, 1)

	got := creds.AWS[0]
	assert.Equal(t, AuthStatic, got.AuthType())
	assert.Equal(t, "AKIA123", got.AccessKey)
}

func TestSaveCredentials_OmitsUnusedFields(t *testing.T) {
	path := t.TempDir() + "/cloud-credentials.yaml"
	creds := &CloudCredentials{AWS: []AWSCredential{
		{Name: "sso", Type: AuthSSO, Profile: "acme-prod", SSOStartURL: "https://acme.awsapps.com/start"},
	}}

	require.NoError(t, creds.Save(path))

	reloaded, err := LoadCredentials(path)
	require.NoError(t, err)
	require.Len(t, reloaded.AWS, 1)
	assert.Equal(t, AuthSSO, reloaded.AWS[0].AuthType())
	assert.Equal(t, "acme-prod", reloaded.AWS[0].Profile)
	assert.Empty(t, reloaded.AWS[0].AccessKey)
	assert.Empty(t, reloaded.AWS[0].SecretKey)
}

func TestAWSCredential_YAMLRoundTrip(t *testing.T) {
	in := AWSCredential{Name: "acme", Type: AuthSSO, Profile: "acme-prod", SSORegion: "us-east-1"}
	data, err := yaml.Marshal(in)
	require.NoError(t, err)

	var out AWSCredential
	require.NoError(t, yaml.Unmarshal(data, &out))
	assert.Equal(t, in, out)
}
