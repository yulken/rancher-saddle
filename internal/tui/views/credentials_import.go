package views

import (
	"errors"
	"os"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/credentials"
	provaws "github.com/Felipalds/rancher-tui/internal/providers/aws"
)

const credentialsPath = "cloud-credentials.yaml"

// loadCredentialsWithDiscovery loads the credential store and, when the store
// does not exist yet, imports every SSO profile found in ~/.aws/config. Without
// this the user can be logged in at the AWS level and still see an empty
// credentials screen, because discovery previously happened only inside the
// credential form.
//
// Import is intentionally limited to the first run: entries the user deletes
// afterwards are not resurrected on the next launch.
func loadCredentialsWithDiscovery(path string) (*credentials.CloudCredentials, int, error) {
	_, statErr := os.Stat(path)
	firstRun := errors.Is(statErr, os.ErrNotExist)

	creds, err := credentials.LoadCredentials(path)
	if err != nil {
		return creds, 0, err
	}
	if !firstRun {
		return creds, 0, nil
	}

	imported := importSSOProfiles(creds)
	if imported > 0 {
		if err := creds.Save(path); err != nil {
			return creds, imported, err
		}
	}

	return creds, imported, nil
}

// importSSOProfiles registers SSO profiles that are not in the store yet and
// returns how many were added. Names and profiles already in use are skipped,
// so a static credential is not shadowed by an SSO profile of the same name.
func importSSOProfiles(creds *credentials.CloudCredentials) int {
	profiles, err := provaws.ListSSOProfiles()
	if err != nil || len(profiles) == 0 {
		return 0
	}

	taken := make(map[string]bool)
	for _, cred := range creds.AWS {
		taken[cred.Name] = true
		if cred.Profile != "" {
			taken[cred.Profile] = true
		}
	}

	imported := 0
	for _, p := range profiles {
		if taken[p.Name] {
			continue
		}
		if err := creds.AddAWSCredential(credentials.AWSCredential{
			Name:          p.Name,
			Type:          core.AuthSSO,
			Profile:       p.Name,
			SSOStartURL:   p.StartURL,
			SSORegion:     p.SSORegion,
			DefaultRegion: p.Region,
		}); err != nil {
			continue
		}
		taken[p.Name] = true
		imported++
	}

	return imported
}
