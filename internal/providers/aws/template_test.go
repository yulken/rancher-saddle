package aws

import (
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderMainTF(t *testing.T, cfg *AWSConfig) string {
	t.Helper()

	tmpl, err := template.ParseFiles(filepath.Join("templates", "main.tf.tmpl"))
	require.NoError(t, err)

	var out strings.Builder
	require.NoError(t, tmpl.Execute(&out, cfg))
	return out.String()
}

func TestMainTemplate_SSOUsesProfileOnly(t *testing.T) {
	out := renderMainTF(t, &AWSConfig{
		AuthMode: "sso",
		Profile:  "acme-prod",
		Region:   "us-east-1",
	})

	assert.Contains(t, out, `profile = "acme-prod"`)
	assert.NotContains(t, out, "access_key")
	assert.NotContains(t, out, "secret_key")
}

func TestMainTemplate_StaticNeverRendersKeys(t *testing.T) {
	out := renderMainTF(t, &AWSConfig{
		AuthMode:  "static",
		AccessKey: "AKIA_LEAKED",
		SecretKey: "leaked-secret",
		Region:    "us-east-1",
	})

	assert.NotContains(t, out, "access_key")
	assert.NotContains(t, out, "secret_key")
	assert.NotContains(t, out, "AKIA_LEAKED")
	assert.NotContains(t, out, "leaked-secret")
	assert.NotContains(t, out, "profile =")
	assert.Contains(t, out, `region  = "us-east-1"`)
}

func TestValidate_AuthModes(t *testing.T) {
	base := map[string]interface{}{
		"region":            "us-east-1",
		"subnet_id":         "subnet-abc",
		"security_group_id": "sg-def",
	}

	t.Run("static requires keys", func(t *testing.T) {
		cfg := map[string]interface{}{}
		for k, v := range base {
			cfg[k] = v
		}
		err := Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access_key")
	})

	t.Run("sso requires profile", func(t *testing.T) {
		cfg := map[string]interface{}{"auth": "sso"}
		for k, v := range base {
			cfg[k] = v
		}
		err := Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "profile")
	})

	t.Run("sso without keys is valid", func(t *testing.T) {
		cfg := map[string]interface{}{"auth": "sso", "profile": "acme-prod"}
		for k, v := range base {
			cfg[k] = v
		}
		assert.NoError(t, Validate(cfg))
	})

	t.Run("static with keys is valid", func(t *testing.T) {
		cfg := map[string]interface{}{"auth": "static", "access_key": "AKIA", "secret_key": "s"}
		for k, v := range base {
			cfg[k] = v
		}
		assert.NoError(t, Validate(cfg))
	})
}
