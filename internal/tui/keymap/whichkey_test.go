package keymap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderWhichKey(t *testing.T) {
	reg := NewRegistry(leader)
	reg.AddAll(DefaultBindings())
	cands := reg.WhichKey([]string{leader}, ModeNormal)

	out := RenderWhichKey(cands, 40)
	assert.NotEmpty(t, out)
	for _, c := range cands {
		assert.Contains(t, out, c.Desc, "popup should list every candidate description")
	}
}

func TestRenderWhichKey_Empty(t *testing.T) {
	assert.Empty(t, RenderWhichKey(nil, 40))
}
