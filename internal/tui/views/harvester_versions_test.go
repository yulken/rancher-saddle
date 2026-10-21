package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHarvesterVersionOptions_Ordering(t *testing.T) {
	m := NewCreateFormModel()
	m.harvesterCached = []string{"v1.4.1"}
	m.harvesterAvailable = []string{"v1.4.1", "v1.4.0", "master"}

	opts := m.harvesterVersionOptions()

	// Cached first (marked), then remaining available (deduped), then Custom last.
	assert.Equal(t, "v1.4.1 (downloaded)", opts[0])
	assert.Equal(t, customVersionOption, opts[len(opts)-1])
	assert.Contains(t, opts, "v1.4.0")
	assert.Contains(t, opts, "master")
	// v1.4.1 should not appear twice (cached dedupes the available entry).
	count := 0
	for _, o := range opts {
		if o == "v1.4.1" {
			count++
		}
	}
	assert.Equal(t, 0, count, "cached version should only appear with the downloaded marker")
}

func TestHarvesterVersionOptions_FallbackWhenNoAvailable(t *testing.T) {
	m := NewCreateFormModel()
	opts := m.harvesterVersionOptions()
	assert.Contains(t, opts, defaultHarvesterVersions[0])
	assert.Equal(t, customVersionOption, opts[len(opts)-1])
}

func TestSelectedHarvesterVersion(t *testing.T) {
	m := NewCreateFormModel()
	m.harvesterCached = []string{"v1.4.1"}
	m.harvesterAvailable = []string{"v1.4.0"}
	m.selectOption(fieldProvider, "libvirt")
	m.coupleProviderAndDistro(fieldProvider) // applies the version picker

	opts := m.fields[fieldOSImage].options

	// Pick the downloaded one -> marker stripped.
	for i, o := range opts {
		if o == "v1.4.1 (downloaded)" {
			m.fields[fieldOSImage].selected = i
		}
	}
	assert.Equal(t, "v1.4.1", m.selectedHarvesterVersion())

	// Pick a plain available version.
	for i, o := range opts {
		if o == "v1.4.0" {
			m.fields[fieldOSImage].selected = i
		}
	}
	assert.Equal(t, "v1.4.0", m.selectedHarvesterVersion())

	// Pick Custom -> reads the custom text field.
	for i, o := range opts {
		if o == customVersionOption {
			m.fields[fieldOSImage].selected = i
		}
	}
	m.fields[fieldCustomAMI].input.SetValue("v9.9.9")
	assert.Equal(t, "v9.9.9", m.selectedHarvesterVersion())
}

func TestDeriveVersionFromISOPath(t *testing.T) {
	assert.Equal(t, "v1.4.1", deriveVersionFromISOPath("/data/harvester-v1.4.1-amd64.iso"))
	assert.Equal(t, "master", deriveVersionFromISOPath("harvester-master-amd64.iso"))
	assert.Equal(t, "", deriveVersionFromISOPath("/data/some-other.iso"))
	assert.Equal(t, "", deriveVersionFromISOPath(""))
}

func TestValueOrAtoiOr(t *testing.T) {
	assert.Equal(t, "x", valueOr("x", "def"))
	assert.Equal(t, "def", valueOr("", "def"))
	assert.Equal(t, "def", valueOr("   ", "def"))

	assert.Equal(t, 8, atoiOr("8", 3))
	assert.Equal(t, 3, atoiOr("", 3))
	assert.Equal(t, 3, atoiOr("abc", 3))
}

func TestVagrantPath_ShowsInstallerFields(t *testing.T) {
	m := NewCreateFormModel()
	m.selectOption(fieldProvider, "libvirt")
	m.coupleProviderAndDistro(fieldProvider)

	for _, idx := range harvesterOnlyFields {
		assert.False(t, m.fields[idx].hidden, "field %d should show on the Harvester path", idx)
	}

	// Back to AWS hides them again.
	m.selectOption(fieldProvider, "AWS")
	m.coupleProviderAndDistro(fieldProvider)
	for _, idx := range harvesterOnlyFields {
		assert.True(t, m.fields[idx].hidden, "field %d should hide on the AWS path", idx)
	}
}

func TestVagrantPath_UsesVersionPicker(t *testing.T) {
	m := NewCreateFormModel()
	m.selectOption(fieldProvider, "libvirt")
	m.coupleProviderAndDistro(fieldProvider)

	// OS Image field is relabeled + visible as the version picker; K8s Version hidden.
	assert.Equal(t, "Harvester Version", m.fields[fieldOSImage].label)
	assert.False(t, m.fields[fieldOSImage].hidden)
	assert.True(t, m.fields[fieldK8sVersion].hidden)

	// Switching back to AWS restores the OS Image label.
	m.selectOption(fieldProvider, "AWS")
	m.coupleProviderAndDistro(fieldProvider)
	assert.Equal(t, "OS Image", m.fields[fieldOSImage].label)
}
