package aws

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvFor_SSO(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA_AMBIENT")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "ambient-secret")
	t.Setenv("AWS_SESSION_TOKEN", "ambient-token")
	t.Setenv("AWS_PROFILE", "some-other-profile")
	t.Setenv("PATH", "/usr/bin")

	cfg := map[string]interface{}{
		"auth":       "sso",
		"profile":    "acme-prod",
		"access_key": "AKIA_CONFIG",
		"secret_key": "config-secret",
		"region":     "us-east-1",
	}

	env := EnvFor(cfg)

	assert.Contains(t, env, "AWS_PROFILE=acme-prod")
	assert.Contains(t, env, "PATH=/usr/bin")
	assert.NotContains(t, env, "AWS_ACCESS_KEY_ID=AKIA_AMBIENT")
	assert.NotContains(t, env, "AWS_ACCESS_KEY_ID=AKIA_CONFIG")
	assert.NotContains(t, env, "AWS_SECRET_ACCESS_KEY=ambient-secret")
	assert.NotContains(t, env, "AWS_SESSION_TOKEN=ambient-token")
	assert.Equal(t, 1, countPrefix(env, "AWS_PROFILE="))
}

func TestEnvFor_Static(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA_AMBIENT")
	t.Setenv("AWS_PROFILE", "leaked-profile")

	cfg := map[string]interface{}{
		"auth":       "static",
		"access_key": "AKIA_CONFIG",
		"secret_key": "config-secret",
	}

	env := EnvFor(cfg)

	assert.Contains(t, env, "AWS_ACCESS_KEY_ID=AKIA_CONFIG")
	assert.Contains(t, env, "AWS_SECRET_ACCESS_KEY=config-secret")
	assert.NotContains(t, env, "AWS_ACCESS_KEY_ID=AKIA_AMBIENT")
	assert.NotContains(t, env, "AWS_PROFILE=leaked-profile")
}

func TestEnvFor_EmptyAuthDefaultsToStatic(t *testing.T) {
	cfg := map[string]interface{}{
		"access_key": "AKIA_CONFIG",
		"secret_key": "config-secret",
	}

	env := EnvFor(cfg)

	assert.Contains(t, env, "AWS_ACCESS_KEY_ID=AKIA_CONFIG")
	assert.False(t, hasPrefix(env, "AWS_PROFILE="))
}

func TestEnvFor_SSOWithoutProfile(t *testing.T) {
	cfg := map[string]interface{}{"auth": "sso"}

	env := EnvFor(cfg)

	assert.False(t, hasPrefix(env, "AWS_PROFILE="))
}

func TestEnvForStatic(t *testing.T) {
	t.Setenv("AWS_PROFILE", "leaked-profile")

	env := EnvForStatic("AKIA_LEGACY", "legacy-secret")

	assert.Contains(t, env, "AWS_ACCESS_KEY_ID=AKIA_LEGACY")
	assert.Contains(t, env, "AWS_SECRET_ACCESS_KEY=legacy-secret")
	assert.NotContains(t, env, "AWS_PROFILE=leaked-profile")
}

func TestEnvForStatic_EmptyKeysAreNotExported(t *testing.T) {
	env := EnvForStatic("", "")

	assert.False(t, hasPrefix(env, "AWS_ACCESS_KEY_ID="))
	assert.False(t, hasPrefix(env, "AWS_SECRET_ACCESS_KEY="))
}

func TestUnset(t *testing.T) {
	env := []string{"PATH=/bin", "AWS_PROFILE=x", "HOME=/root", "AWS_PROFILE=y"}

	got := unset(env, "AWS_PROFILE")

	assert.Equal(t, []string{"PATH=/bin", "HOME=/root"}, got)
}

func hasPrefix(env []string, prefix string) bool {
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

func countPrefix(env []string, prefix string) int {
	n := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			n++
		}
	}
	return n
}
