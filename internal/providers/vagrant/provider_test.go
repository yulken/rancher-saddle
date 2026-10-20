package vagrant

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestVagrantProvider_Name(t *testing.T) {
	assert.Equal(t, core.ProviderVagrant, NewProvider().Name())
}

func TestVagrantProvider_GenerateInfrastructure_NotImplemented(t *testing.T) {
	p := NewProvider()
	dir := t.TempDir()

	err := p.GenerateInfrastructure(context.Background(), GetDefaultConfig(), dir)
	assert.True(t, errors.Is(err, ErrNotImplemented))

	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries, "scaffold must not write any files")
}

func TestVagrantProvider_GetOutputs_NotImplemented(t *testing.T) {
	_, err := NewProvider().GetOutputs(context.Background(), t.TempDir())
	assert.True(t, errors.Is(err, ErrNotImplemented))
}

func TestVagrantProvider_Validate(t *testing.T) {
	p := NewProvider()
	assert.NoError(t, p.Validate(GetDefaultConfig()))
	assert.Error(t, p.Validate(map[string]interface{}{"backend": "virtualbox"}))
}

func TestVagrantProvider_SatisfiesInterface(t *testing.T) {
	var _ core.Provider = NewProvider()
}
