package aws

import (
	"os"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/core"
)

// awsEnvKeys are the environment variables that can inject (or override) AWS
// credentials into a child process. They are always scrubbed before rebuilding
// the environment, because static credentials in the environment take precedence
// over `profile` in the AWS SDK — the classic "works in my shell, fails from the
// TUI" failure.
var awsEnvKeys = []string{
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
	"AWS_PROFILE",
	"AWS_DEFAULT_PROFILE",
}

// AuthMode returns the authentication mode declared in a provider config map.
// Anything other than an explicit "sso" is treated as static, which keeps
// pre-existing clusters working unchanged.
func AuthMode(cfg map[string]interface{}) string {
	if cfg == nil {
		return core.AuthStatic
	}
	if mode, _ := cfg["auth"].(string); mode == core.AuthSSO {
		return core.AuthSSO
	}
	return core.AuthStatic
}

// Profile returns the SSO profile name from a provider config map.
func Profile(cfg map[string]interface{}) string {
	if cfg == nil {
		return ""
	}
	v, _ := cfg["profile"].(string)
	return v
}

// EnvFor builds the environment for AWS-authenticated child processes (`tofu`).
// In SSO mode the only credential-related value is AWS_PROFILE; in static mode
// the key pair is exported so it never has to be rendered into a .tf file.
func EnvFor(cfg map[string]interface{}) []string {
	env := unset(os.Environ(), awsEnvKeys...)

	if AuthMode(cfg) == core.AuthSSO {
		if profile := Profile(cfg); profile != "" {
			env = append(env, "AWS_PROFILE="+profile)
		}
		return env
	}

	return appendStaticEnv(env, stringValue(cfg, "access_key"), stringValue(cfg, "secret_key"))
}

// EnvForStatic is the equivalent of EnvFor for the legacy model.Config path,
// which carries credentials as struct fields instead of a config map.
func EnvForStatic(accessKey, secretKey string) []string {
	env := unset(os.Environ(), awsEnvKeys...)
	return appendStaticEnv(env, accessKey, secretKey)
}

func appendStaticEnv(env []string, accessKey, secretKey string) []string {
	if accessKey != "" {
		env = append(env, "AWS_ACCESS_KEY_ID="+accessKey)
	}
	if secretKey != "" {
		env = append(env, "AWS_SECRET_ACCESS_KEY="+secretKey)
	}
	return env
}

// unset removes every KEY=VALUE entry whose KEY is in keys, preserving order.
func unset(env []string, keys ...string) []string {
	out := env[:0:0]
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if contains(keys, name) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func stringValue(cfg map[string]interface{}, key string) string {
	if cfg == nil {
		return ""
	}
	v, _ := cfg[key].(string)
	return v
}
