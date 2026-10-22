package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrSSOExpired means the cached SSO token is no longer usable and the user
	// must authenticate again with `aws sso login --profile <name>`.
	ErrSSOExpired = errors.New("AWS SSO session expired")

	// ErrSSONotLoggedIn means no cached token exists for the profile yet.
	ErrSSONotLoggedIn = errors.New("AWS SSO login required")

	// ErrSSOCLIUnavailable means the AWS CLI v2 binary is not on PATH.
	ErrSSOCLIUnavailable = errors.New("AWS CLI v2 is required for SSO authentication")

	// ErrUnknownProfile means ~/.aws/config has no such profile.
	ErrUnknownProfile = errors.New("AWS profile not found")
)

// SSOProfile describes a profile from ~/.aws/config that authenticates via IAM
// Identity Center (SSO). Both the legacy `sso_start_url` form and the newer
// `sso_session` indirection are supported.
type SSOProfile struct {
	Name        string
	StartURL    string
	SSORegion   string
	AccountID   string
	RoleName    string
	SessionName string
	// Region is the default AWS region configured on the profile (deploy region),
	// which is independent from the Identity Center region (SSORegion).
	Region string
}

// CallerIdentity is the subset of sts:GetCallerIdentity we display.
type CallerIdentity struct {
	Account string
	ARN     string
	UserID  string
}

// runAWS executes the AWS CLI. It is a package variable so tests can substitute
// a fake without shelling out.
var runAWS = func(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "aws", args...)
	return cmd.CombinedOutput()
}

// DefaultAWSConfigPath returns $AWS_CONFIG_FILE or ~/.aws/config.
func DefaultAWSConfigPath() string {
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aws", "config")
}

// ssoCacheDir returns the directory where the AWS CLI caches Identity Center
// access tokens. It is a variable so tests can point at a fixture directory.
var ssoCacheDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aws", "sso", "cache")
}

// SSOTokenExpiry returns when the cached access token for a profile expires.
// found is false when no usable token is cached (not logged in) or when the
// profile is not reachable from the cache.
//
// Several entries can exist for the same Identity Center instance (one per
// login). The newest access token wins; registration-only entries, which carry
// no startUrl/accessToken, are ignored.
func SSOTokenExpiry(profile SSOProfile) (expiry time.Time, found bool) {
	dir := ssoCacheDir()
	if dir == "" || profile.StartURL == "" {
		return time.Time{}, false
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, false
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var token struct {
			StartURL    string `json:"startUrl"`
			Region      string `json:"region"`
			AccessToken string `json:"accessToken"`
			ExpiresAt   string `json:"expiresAt"`
		}
		if err := json.Unmarshal(data, &token); err != nil {
			continue
		}
		if token.AccessToken == "" || token.StartURL != profile.StartURL {
			continue
		}
		if profile.SSORegion != "" && token.Region != profile.SSORegion {
			continue
		}

		exp, err := parseCacheTime(token.ExpiresAt)
		if err != nil {
			continue
		}
		if !found || exp.After(expiry) {
			expiry, found = exp, true
		}
	}

	return expiry, found
}

// parseCacheTime accepts both timestamp shapes written to the SSO cache:
// RFC3339 (AWS CLI v2) and the older botocore "...UTC" suffix.
func parseCacheTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if trimmed, ok := strings.CutSuffix(value, "UTC"); ok {
		return time.Parse("2006-01-02T15:04:05", trimmed)
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp %q", value)
}

// ListSSOProfiles returns every SSO-capable profile in the default config file.
// A missing file is not an error: it just means there are no profiles to pick.
func ListSSOProfiles() ([]SSOProfile, error) {
	return ListSSOProfilesFrom(DefaultAWSConfigPath())
}

// ListSSOProfilesFrom is ListSSOProfiles with an explicit config path (tests).
func ListSSOProfilesFrom(path string) ([]SSOProfile, error) {
	sections, err := parseINI(path)
	if err != nil {
		return nil, err
	}

	sessions := make(map[string]map[string]string)
	for _, section := range sections {
		if name, ok := strings.CutPrefix(section.Name, "sso-session "); ok {
			sessions[strings.TrimSpace(name)] = section.Values
		}
	}

	var profiles []SSOProfile
	for _, section := range sections { // preserve file order for stable UI
		name := section.Name
		switch {
		case strings.HasPrefix(name, "sso-session "):
			continue
		case strings.HasPrefix(name, "profile "):
			name = strings.TrimSpace(strings.TrimPrefix(name, "profile "))
		case name != "default":
			continue
		}

		profile := SSOProfile{
			Name:        name,
			AccountID:   section.Values["sso_account_id"],
			RoleName:    section.Values["sso_role_name"],
			SessionName: section.Values["sso_session"],
			StartURL:    section.Values["sso_start_url"],
			SSORegion:   section.Values["sso_region"],
			Region:      section.Values["region"],
		}

		if profile.SessionName != "" {
			session := sessions[profile.SessionName]
			profile.StartURL = session["sso_start_url"]
			profile.SSORegion = session["sso_region"]
		}

		// Profiles without a start URL (directly or via sso_session) are not SSO.
		if profile.StartURL == "" {
			continue
		}
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// FindSSOProfile returns the named profile, if it is configured for SSO.
func FindSSOProfile(name string) (*SSOProfile, error) {
	profiles, err := ListSSOProfiles()
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].Name == name {
			return &profiles[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownProfile, name)
}

// Identity validates the credentials for a profile by calling
// sts:GetCallerIdentity. That action is always allowed and costs one API call,
// which makes it a reliable preflight.
func Identity(ctx context.Context, profile string) (*CallerIdentity, error) {
	if profile == "" {
		return nil, fmt.Errorf("AWS SSO profile is required")
	}

	out, err := runAWS(ctx, "sts", "get-caller-identity", "--profile", profile, "--output", "json")
	if err != nil {
		return nil, classifyAWSError(string(out), err)
	}

	var payload struct {
		Account string `json:"Account"`
		ARN     string `json:"Arn"`
		UserID  string `json:"UserId"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parsing STS response: %w", err)
	}

	return &CallerIdentity{Account: payload.Account, ARN: payload.ARN, UserID: payload.UserID}, nil
}

// CheckSession is the provider preflight: nil when the cluster configuration
// does not use SSO, otherwise it verifies the profile can authenticate.
func CheckSession(ctx context.Context, cfg map[string]interface{}) error {
	if AuthMode(cfg) != "sso" {
		return nil
	}
	profile := Profile(cfg)
	if profile == "" {
		return fmt.Errorf("AWS config: 'profile' is required when auth is 'sso'")
	}
	_, err := Identity(ctx, profile)
	return err
}

// DescribeSession renders a short status for the credentials list.
func DescribeSession(ctx context.Context, profile string) string {
	identity, err := Identity(ctx, profile)
	if err == nil {
		account := identity.Account
		if account == "" {
			account = "-"
		}
		return fmt.Sprintf("✓ valid (acct %s)", account)
	}

	switch {
	case errors.Is(err, ErrSSOExpired):
		return "✗ expired"
	case errors.Is(err, ErrSSONotLoggedIn):
		return "✗ not logged in"
	case errors.Is(err, ErrSSOCLIUnavailable):
		return "✗ aws cli missing"
	case errors.Is(err, ErrUnknownProfile):
		return "✗ profile not found"
	default:
		return "? unknown"
	}
}

// classifyAWSError maps AWS CLI output/errors onto the sentinel errors above.
func classifyAWSError(output string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%w: install AWS CLI v2 and ensure `aws` is on PATH", ErrSSOCLIUnavailable)
	}

	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "sso session") && strings.Contains(lower, "expired"),
		strings.Contains(lower, "token has expired"),
		strings.Contains(lower, "session has expired"):
		return fmt.Errorf("%w: %s", ErrSSOExpired, firstLine(output))
	case strings.Contains(lower, "error loading sso token"),
		strings.Contains(lower, "token for") && strings.Contains(lower, "does not exist"),
		strings.Contains(lower, "unable to locate credentials"):
		return fmt.Errorf("%w: %s", ErrSSONotLoggedIn, firstLine(output))
	case strings.Contains(lower, "could not be found"),
		strings.Contains(lower, "profile") && strings.Contains(lower, "not found"):
		return fmt.Errorf("%w: %s", ErrUnknownProfile, firstLine(output))
	default:
		return fmt.Errorf("AWS authentication failed: %s", firstLine(output))
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if line, _, ok := strings.Cut(s, "\n"); ok {
		return strings.TrimSpace(line)
	}
	return s
}

// iniSection is one `[name]` block of an AWS config file.
type iniSection struct {
	Name   string
	Values map[string]string
}

// parseINI reads the subset of INI syntax used by ~/.aws/config. A missing file
// returns an empty result, not an error.
func parseINI(path string) ([]iniSection, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading AWS config %s: %w", path, err)
	}

	var sections []iniSection
	current := -1

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections = append(sections, iniSection{
				Name:   strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")),
				Values: make(map[string]string),
			})
			current = len(sections) - 1
			continue
		}

		if current < 0 {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		sections[current].Values[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}

	return sections, nil
}
