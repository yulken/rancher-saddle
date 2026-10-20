package keymap

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const leader = " " // space, Astronvim-style

func newTestResolver() *Resolver {
	reg := NewRegistry(leader)
	reg.AddAll(DefaultBindings())
	return NewResolver(reg, time.Second)
}

func TestResolver_SingleKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want CommandID
	}{
		{name: "j navigates down", key: "j", want: CmdNavDown},
		{name: "k navigates up", key: "k", want: CmdNavUp},
		{name: "G jumps to bottom", key: "G", want: CmdNavBottom},
		{name: "? toggles help", key: "?", want: CmdHelpToggle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := newTestResolver()
			got := res.Feed(tt.key, time.Now())
			assert.Equal(t, Resolved, got.Status)
			assert.Equal(t, tt.want, got.ID)
			assert.Empty(t, res.Pending(), "pending should clear after resolve")
		})
	}
}

func TestResolver_MultiKeySequence(t *testing.T) {
	res := newTestResolver()
	now := time.Now()

	first := res.Feed("g", now)
	assert.Equal(t, Pending, first.Status, "single g is a prefix of gg")
	assert.Equal(t, []string{"g"}, res.Pending())

	second := res.Feed("g", now)
	assert.Equal(t, Resolved, second.Status)
	assert.Equal(t, CmdNavTop, second.ID)
	assert.Empty(t, res.Pending())
}

func TestResolver_LeaderExpansion(t *testing.T) {
	res := newTestResolver()
	now := time.Now()

	r1 := res.Feed(leader, now)
	assert.Equal(t, Pending, r1.Status)
	assert.GreaterOrEqual(t, len(r1.Pending), 2, "leader shows which-key candidates")

	r2 := res.Feed("f", now)
	assert.Equal(t, Pending, r2.Status)

	r3 := res.Feed("f", now)
	assert.Equal(t, Resolved, r3.Status)
	assert.Equal(t, CmdClusterCreate, r3.ID)
}

func TestResolver_NoMatch(t *testing.T) {
	res := newTestResolver()
	now := time.Now()

	assert.Equal(t, Pending, res.Feed("g", now).Status)

	got := res.Feed("z", now)
	assert.Equal(t, NoMatch, got.Status)
	assert.Empty(t, res.Pending(), "pending resets after a no-match")
}

func TestResolver_Timeout(t *testing.T) {
	res := newTestResolver()
	start := time.Now()

	assert.Equal(t, Pending, res.Feed("g", start).Status)

	// Second g arrives after the timeout window -> treated as a fresh g, not gg.
	got := res.Feed("g", start.Add(2*time.Second))
	assert.Equal(t, Pending, got.Status)
	assert.Equal(t, []string{"g"}, res.Pending())
}

func TestResolver_Reset(t *testing.T) {
	res := newTestResolver()
	res.Feed("g", time.Now())
	assert.NotEmpty(t, res.Pending())

	res.Reset()
	assert.Empty(t, res.Pending())
}

func TestResolver_ModeSwitch(t *testing.T) {
	res := newTestResolver()
	now := time.Now()

	assert.Equal(t, ModeNormal, res.Mode())

	res.Feed(":", now)
	assert.Equal(t, ModeCommand, res.Mode())

	res.Feed("esc", now)
	assert.Equal(t, ModeNormal, res.Mode())

	res.Feed("/", now)
	assert.Equal(t, ModeSearch, res.Mode())

	res.Feed("esc", now)
	assert.Equal(t, ModeNormal, res.Mode())
}

func TestRegistry_WhichKey(t *testing.T) {
	reg := NewRegistry(leader)
	reg.AddAll(DefaultBindings())

	cands := reg.WhichKey([]string{leader}, ModeNormal)
	assert.GreaterOrEqual(t, len(cands), 2)
	for _, c := range cands {
		assert.Equal(t, leader, c.Keys[0], "candidate must be leader-rooted")
		assert.NotEmpty(t, c.Desc, "which-key entries need a description")
	}
}

func TestDefaultBindings_NoCollisions(t *testing.T) {
	reg := NewRegistry(leader)
	reg.AddAll(DefaultBindings())

	seen := map[string]bool{}
	for _, b := range DefaultBindings() {
		// Expand leader the same way the registry does before checking.
		expanded := make([]string, len(b.Keys))
		for i, k := range b.Keys {
			if k == LeaderToken {
				expanded[i] = leader
			} else {
				expanded[i] = k
			}
		}
		id := string(rune(b.Mode)) + "|" + strings.Join(expanded, ",")
		assert.False(t, seen[id], "duplicate binding for mode/keys %q", id)
		seen[id] = true
	}
}
