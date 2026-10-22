package views

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/credentials"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatRemaining(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"expired", -time.Minute, "expired"},
		{"zero", 0, "expired"},
		{"seconds", 42 * time.Second, "expires in less than a minute"},
		{"minutes", 42 * time.Minute, "expires in 42m"},
		{"hours", 7*time.Hour + 12*time.Minute, "expires in 7h12m"},
		{"just under an hour", 59*time.Minute + 59*time.Second, "expires in 59m"},
		{"pads minutes", 8*time.Hour + 5*time.Minute, "expires in 8h05m"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatRemaining(tc.d))
		})
	}
}

func newCredentialsListForTest() CredentialsListModel {
	m := NewCredentialsListModel()
	m.credentials = &credentials.CloudCredentials{AWS: []credentials.AWSCredential{
		{
			Name:      "acme-prod",
			Type:      core.AuthSSO,
			Profile:   "acme-prod",
			SSORegion: "us-west-2",
		},
		{
			Name:      "legacy",
			AccessKey: "AKIA",
			SecretKey: "secret",
		},
	}}
	m.credNames = m.credentials.ListAWSCredentials()
	m.updateTable()
	return m
}

func TestCredentialsList_DetailLineShowsTokenExpiry(t *testing.T) {
	m := newCredentialsListForTest()
	m.table.SetCursor(0) // "acme-prod"
	m.expiries["acme-prod"] = time.Now().Add(7*time.Hour + 12*time.Minute + 30*time.Second)

	line := m.detailLine()

	assert.Contains(t, line, "profile acme-prod")
	assert.Contains(t, line, "identity center us-west-2")
	assert.Contains(t, line, "token expires in 7h12m")
	assert.NotContains(t, line, "log in")
}

func TestCredentialsList_DetailLineWithoutCachedToken(t *testing.T) {
	m := newCredentialsListForTest()
	m.table.SetCursor(0)

	line := m.detailLine()

	assert.Contains(t, line, "profile acme-prod")
	assert.Contains(t, line, "no cached token")
}

func TestCredentialsList_DetailLineExpiredToken(t *testing.T) {
	m := newCredentialsListForTest()
	m.table.SetCursor(0)
	m.expiries["acme-prod"] = time.Now().Add(-2 * time.Hour)

	assert.Contains(t, m.detailLine(), "token expired")
}

func TestCredentialsList_DetailLineEmptyForStaticCredentials(t *testing.T) {
	m := newCredentialsListForTest()
	m.table.SetCursor(1) // "legacy", static keys

	assert.Empty(t, m.detailLine())
}

func TestCredentialsList_ViewRendersDetailLine(t *testing.T) {
	m := newCredentialsListForTest()
	m.table.SetCursor(0)
	m.expiries["acme-prod"] = time.Now().Add(3*time.Hour + 5*time.Minute + 30*time.Second)

	view := m.View()

	assert.Contains(t, view, "token expires in 3h05m")
}

const testSSOCredentialsYAML = `aws:
  - name: acme-prod
    type: sso
    profile: acme-prod
    sso_region: us-west-2
    default_region: us-east-1
`

func writeTestCredentials(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cloud-credentials.yaml"), []byte(body), 0600))
}

func TestCredentialsList_RefreshKeyReloadsAndReprobes(t *testing.T) {
	writeTestCredentials(t, testSSOCredentialsYAML)

	// State gathered when the screen was opened, now stale: a credential that
	// no longer exists on disk, a probe result and an untouched expiry.
	m := NewCredentialsListModel()
	m.credentials = &credentials.CloudCredentials{AWS: []credentials.AWSCredential{{Name: "removed"}}}
	m.credNames = []string{"removed"}
	m.statuses = map[string]string{"acme-prod": "valid (acct 123456789012)"}
	m.expiries = map[string]time.Time{"acme-prod": time.Now().Add(-time.Hour)}
	m.updateTable()

	refreshed, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	require.NotNil(t, cmd, "refresh must reload the credentials from disk")
	assert.Equal(t, refreshingNotice, refreshed.notice)
	assert.Empty(t, refreshed.statuses, "stale probe results must not survive a refresh")
	assert.Empty(t, refreshed.expiries, "stale token expiries must not survive a refresh")

	// Loading answers with the file on disk; the handler then schedules the probe.
	msg, ok := cmd().(credentialsLoadedMsg)
	require.True(t, ok)
	loaded, probe := refreshed.Update(msg)

	require.NotNil(t, probe, "refresh must re-probe the SSO session")
	assert.Equal(t, []string{"acme-prod"}, loaded.credNames)
	assert.Equal(t, "checking…", loaded.table.Rows()[0][3])
	assert.Contains(t, loaded.table.Rows()[0][1], "AWS SSO")

	// The probe result ends the refresh: results land and the notice clears.
	done, cmd := loaded.Update(ssoStatusMsg{
		statuses: map[string]string{"acme-prod": "valid (acct 123456789012)"},
		expiries: map[string]time.Time{"acme-prod": time.Now().Add(2 * time.Hour)},
	})

	assert.Nil(t, cmd)
	assert.Empty(t, done.notice)
	assert.Equal(t, "valid (acct 123456789012)", done.statuses["acme-prod"])
	assert.Contains(t, done.detailLine(), "token expires in")
}

func TestCredentialsList_RefreshClearsNoticeWithoutSSOCredentials(t *testing.T) {
	m := newCredentialsListForTest()
	m.notice = refreshingNotice

	done, cmd := m.Update(credentialsLoadedMsg{
		credentials: &credentials.CloudCredentials{AWS: []credentials.AWSCredential{
			{Name: "legacy", AccessKey: "AKIA", SecretKey: "secret"},
		}},
		names: []string{"legacy"},
	})

	assert.Nil(t, cmd, "static credentials need no probe")
	assert.Empty(t, done.notice, "the refresh notice must not get stuck")
}

func TestCredentialsList_StatusResultClearsRefreshNoticeOnly(t *testing.T) {
	m := newCredentialsListForTest()
	m.notice = "Other notice."

	done, _ := m.Update(ssoStatusMsg{statuses: map[string]string{"acme-prod": "valid (acct 1)"}})

	assert.Equal(t, "Other notice.", done.notice, "only the refresh placeholder is cleared")
}
