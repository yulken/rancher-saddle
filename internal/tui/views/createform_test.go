package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// providerOptionIndex returns the index of the first provider option containing
// the given substring, or -1 if none matches.
func providerOptionIndex(m *CreateFormModel, substr string) int {
	for i, opt := range m.fields[fieldProvider].options {
		if strings.Contains(opt, substr) {
			return i
		}
	}
	return -1
}

func TestCreateForm_CredentialsHiddenForVagrant(t *testing.T) {
	m := NewCreateFormModel()

	awsIdx := providerOptionIndex(&m, "AWS")
	vagrantIdx := providerOptionIndex(&m, "libvirt")
	assert.GreaterOrEqual(t, awsIdx, 0, "AWS provider option must exist")
	assert.GreaterOrEqual(t, vagrantIdx, 0, "Vagrant provider option must exist")

	tests := []struct {
		name       string
		provider   int
		wantHidden bool
	}{
		{"AWS keeps credentials visible", awsIdx, false},
		{"Vagrant hides credentials", vagrantIdx, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.fields[fieldProvider].selected = tt.provider
			m.syncProviderVisibility()
			assert.Equal(t, tt.wantHidden, m.fields[fieldCredentials].hidden)
		})
	}
}

func TestCreateForm_DefaultProviderShowsCredentials(t *testing.T) {
	// Default selection is AWS, so Credentials must be visible out of the box.
	m := NewCreateFormModel()
	assert.False(t, m.isLocalProvider(), "default provider should not be Vagrant")
	assert.False(t, m.fields[fieldCredentials].hidden, "credentials visible by default")
}

func TestCreateForm_VagrantHidesAWSFields(t *testing.T) {
	m := NewCreateFormModel()
	m.fields[fieldProvider].selected = providerOptionIndex(&m, "libvirt")
	m.syncProviderVisibility()

	for _, idx := range awsOnlyFields {
		assert.True(t, m.fields[idx].hidden, "field %d must be hidden for Vagrant", idx)
	}
	assert.True(t, m.fields[fieldInstallMethod].hidden, "install method hidden for Vagrant")
	// Cluster name / node prefix / count stay visible.
	assert.False(t, m.fields[fieldClusterName].hidden)
	assert.False(t, m.fields[fieldNodePrefix].hidden)
	assert.False(t, m.fields[fieldInstanceCount].hidden)
}

func TestCreateForm_AWSRestoresFields(t *testing.T) {
	m := NewCreateFormModel()
	// Go to Vagrant then back to AWS.
	m.fields[fieldProvider].selected = providerOptionIndex(&m, "libvirt")
	m.syncProviderVisibility()
	m.fields[fieldProvider].selected = providerOptionIndex(&m, "AWS")
	m.syncProviderVisibility()

	assert.False(t, m.fields[fieldCredentials].hidden)
	assert.False(t, m.fields[fieldRegion].hidden)
	assert.False(t, m.fields[fieldInstanceType].hidden)
}

func TestCreateForm_CoupleProviderAndDistro(t *testing.T) {
	m := NewCreateFormModel()

	// Selecting Vagrant forces distro to Harvester.
	m.fields[fieldProvider].selected = providerOptionIndex(&m, "libvirt")
	m.coupleProviderAndDistro(fieldProvider)
	assert.True(t, m.isHarvesterDistro(), "Vagrant should force Harvester distro")

	// Switching distro back to RKE2 forces provider to AWS.
	m.selectOption(fieldK8sDistro, "RKE2")
	m.coupleProviderAndDistro(fieldK8sDistro)
	assert.False(t, m.isLocalProvider(), "RKE2 should force AWS provider")

	// Selecting Harvester forces provider to Vagrant.
	m.selectOption(fieldK8sDistro, "Harvester")
	m.coupleProviderAndDistro(fieldK8sDistro)
	assert.True(t, m.isLocalProvider(), "Harvester should force Vagrant provider")
}

func TestCreateForm_IsVagrantProvider(t *testing.T) {
	m := NewCreateFormModel()
	vagrantIdx := providerOptionIndex(&m, "libvirt")
	assert.GreaterOrEqual(t, vagrantIdx, 0)

	m.fields[fieldProvider].selected = vagrantIdx
	assert.True(t, m.isLocalProvider())

	awsIdx := providerOptionIndex(&m, "AWS")
	m.fields[fieldProvider].selected = awsIdx
	assert.False(t, m.isLocalProvider())
}
