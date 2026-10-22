package aws

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAWSConfig = `# comment
[default]
region = us-east-1

[profile acme-prod]
sso_session = acme
sso_account_id = 111122223333
sso_role_name = AdministratorAccess
region = us-east-1

[profile legacy-sso]
sso_start_url = https://legacy.awsapps.com/start
sso_region = us-west-2
sso_account_id = 444455556666
sso_role_name = ReadOnly

[profile plain-static]
region = eu-west-1

[sso-session acme]
sso_start_url = https://acme.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
`

func TestListSSOProfilesFrom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(testAWSConfig), 0600))

	profiles, err := ListSSOProfilesFrom(path)
	require.NoError(t, err)
	require.Len(t, profiles, 2)

	assert.Equal(t, SSOProfile{
		Name:        "acme-prod",
		StartURL:    "https://acme.awsapps.com/start",
		SSORegion:   "us-east-1",
		AccountID:   "111122223333",
		RoleName:    "AdministratorAccess",
		SessionName: "acme",
		Region:      "us-east-1",
	}, profiles[0])

	assert.Equal(t, SSOProfile{
		Name:      "legacy-sso",
		StartURL:  "https://legacy.awsapps.com/start",
		SSORegion: "us-west-2",
		AccountID: "444455556666",
		RoleName:  "ReadOnly",
	}, profiles[1])
}

func TestListSSOProfilesFrom_MissingFile(t *testing.T) {
	profiles, err := ListSSOProfilesFrom(filepath.Join(t.TempDir(), "nope"))

	assert.NoError(t, err)
	assert.Empty(t, profiles)
}

func TestListSSOProfilesFrom_EmptyPath(t *testing.T) {
	profiles, err := ListSSOProfilesFrom("")

	assert.NoError(t, err)
	assert.Empty(t, profiles)
}

func TestClassifyAWSError(t *testing.T) {
	tests := []struct {
		name   string
		output string
		err    error
		want   error
	}{
		{
			name: "cli missing",
			err:  exec.ErrNotFound,
			want: ErrSSOCLIUnavailable,
		},
		{
			name:   "session expired",
			output: "The SSO session associated with this profile has expired or is otherwise invalid. To refresh this SSO session run aws sso login with the corresponding profile.",
			err:    errors.New("exit status 255"),
			want:   ErrSSOExpired,
		},
		{
			name:   "not logged in",
			output: "Error loading SSO Token: Token for acme does not exist",
			err:    errors.New("exit status 255"),
			want:   ErrSSONotLoggedIn,
		},
		{
			name:   "unknown profile",
			output: "The config profile (nope) could not be found",
			err:    errors.New("exit status 255"),
			want:   ErrUnknownProfile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyAWSError(tt.output, tt.err)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestDescribeSession(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T)
		want  string
	}{
		{
			name: "valid",
			setup: func(t *testing.T) {
				stubAWS(t, []byte(`{"UserId":"AIDA","Account":"123456789012","Arn":"arn:aws:sts::123456789012:assumed-role/x/y"}`), nil)
			},
			want: "✓ valid (acct 123456789012)",
		},
		{
			name: "expired",
			setup: func(t *testing.T) {
				stubAWS(t, []byte("The SSO session associated with this profile has expired"), errors.New("exit status 255"))
			},
			want: "✗ expired",
		},
		{
			name: "cli missing",
			setup: func(t *testing.T) {
				stubAWS(t, nil, exec.ErrNotFound)
			},
			want: "✗ aws cli missing",
		},
		{
			name: "not logged in",
			setup: func(t *testing.T) {
				stubAWS(t, []byte("Error loading SSO Token: Token for acme does not exist"), errors.New("exit status 255"))
			},
			want: "✗ not logged in",
		},
		{
			name: "unknown profile",
			setup: func(t *testing.T) {
				stubAWS(t, []byte("The config profile (x) could not be found"), errors.New("exit status 255"))
			},
			want: "✗ profile not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			assert.Equal(t, tt.want, DescribeSession(context.Background(), "acme-prod"))
		})
	}
}

func TestCheckSession(t *testing.T) {
	called := false
	stub := func(ctx context.Context, args ...string) ([]byte, error) {
		called = true
		return []byte(`{"Account":"123456789012"}`), nil
	}
	original := runAWS
	runAWS = stub
	t.Cleanup(func() { runAWS = original })

	t.Run("static does not call the CLI", func(t *testing.T) {
		called = false
		err := CheckSession(context.Background(), map[string]interface{}{"auth": "static", "access_key": "a", "secret_key": "b"})
		assert.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("sso requires a profile", func(t *testing.T) {
		err := CheckSession(context.Background(), map[string]interface{}{"auth": "sso"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "profile")
	})

	t.Run("sso calls the CLI", func(t *testing.T) {
		called = false
		err := CheckSession(context.Background(), map[string]interface{}{"auth": "sso", "profile": "acme-prod"})
		assert.NoError(t, err)
		assert.True(t, called)
	})
}

func TestFindSSOProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(testAWSConfig), 0600))
	t.Setenv("AWS_CONFIG_FILE", path)

	got, err := FindSSOProfile("legacy-sso")
	require.NoError(t, err)
	assert.Equal(t, "https://legacy.awsapps.com/start", got.StartURL)

	_, err = FindSSOProfile("plain-static")
	assert.ErrorIs(t, err, ErrUnknownProfile)
}

// stubAWS replaces the CLI runner for the duration of the test.
func stubAWS(t *testing.T, output []byte, err error) {
	t.Helper()
	original := runAWS
	runAWS = func(ctx context.Context, args ...string) ([]byte, error) {
		return output, err
	}
	t.Cleanup(func() { runAWS = original })
}

func TestSSOTokenExpiry_PicksNewestAccessToken(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0600))
	}

	// Two logins for the same Identity Center instance: only the newest counts.
	write("old.json", `{"startUrl":"https://example.awsapps.com/start","region":"us-west-2","accessToken":"old","expiresAt":"2026-08-31T19:15:11Z"}`)
	write("new.json", `{"startUrl":"https://example.awsapps.com/start","region":"us-west-2","accessToken":"new","expiresAt":"2026-10-22T15:24:25Z"}`)
	// Client registration entry: no startUrl/accessToken, must be ignored.
	write("registration.json", `{"clientId":"x","clientSecret":"y","expiresAt":"2026-12-03T13:30:35Z"}`)
	// Same start URL, different Identity Center region.
	write("other-region.json", `{"startUrl":"https://example.awsapps.com/start","region":"eu-west-1","accessToken":"other","expiresAt":"2027-01-01T00:00:00Z"}`)
	// Another instance entirely.
	write("other-instance.json", `{"startUrl":"https://other.awsapps.com/start","region":"us-west-2","accessToken":"x","expiresAt":"2027-01-01T00:00:00Z"}`)

	restore := ssoCacheDir
	ssoCacheDir = func() string { return dir }
	t.Cleanup(func() { ssoCacheDir = restore })

	expiry, found := SSOTokenExpiry(SSOProfile{
		Name:      "acme-prod",
		StartURL:  "https://example.awsapps.com/start",
		SSORegion: "us-west-2",
	})

	require.True(t, found)
	assert.Equal(t, time.Date(2026, 10, 22, 15, 24, 25, 0, time.UTC), expiry.UTC())
}

func TestSSOTokenExpiry_NoToken(t *testing.T) {
	restore := ssoCacheDir
	ssoCacheDir = func() string { return filepath.Join(t.TempDir(), "missing") }
	t.Cleanup(func() { ssoCacheDir = restore })

	_, found := SSOTokenExpiry(SSOProfile{StartURL: "https://example.awsapps.com/start"})
	assert.False(t, found)
}

func TestSSOTokenExpiry_RequiresStartURL(t *testing.T) {
	_, found := SSOTokenExpiry(SSOProfile{})
	assert.False(t, found)
}

func TestParseCacheTime(t *testing.T) {
	t.Run("rfc3339", func(t *testing.T) {
		got, err := parseCacheTime("2026-10-22T15:24:25Z")
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 22, 15, 24, 25, 0, time.UTC), got.UTC())
	})

	t.Run("botocore UTC suffix", func(t *testing.T) {
		got, err := parseCacheTime("2026-10-22T15:24:25UTC")
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 22, 15, 24, 25, 0, time.UTC), got.UTC())
	})

	t.Run("garbage", func(t *testing.T) {
		_, err := parseCacheTime("tomorrow")
		require.Error(t, err)
	})
}
