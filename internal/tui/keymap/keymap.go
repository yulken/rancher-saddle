// Package keymap implements an nvim-style key-sequence resolution engine.
//
// It is deliberately free of any bubbletea dependency so the resolution logic
// (multi-key sequences, leader expansion, pending state, timeouts, modal
// switching) can be unit-tested as pure logic. The TUI layer feeds raw key
// strings into a Resolver and acts on the returned CommandID.
package keymap

import (
	"time"
)

// LeaderToken is the placeholder used inside a Binding's Keys to stand in for
// the configured leader key (Astronvim-style, default space). The Registry
// expands it to the real leader when a binding is added.
const LeaderToken = "<leader>"

// CommandID is a stable identifier an action maps to, decoupled from tea.Msg.
type CommandID string

// Built-in command identifiers emitted by the default binding set.
const (
	CmdNavDown       CommandID = "nav.down"
	CmdNavUp         CommandID = "nav.up"
	CmdNavTop        CommandID = "nav.top"
	CmdNavBottom     CommandID = "nav.bottom"
	CmdNavBack       CommandID = "nav.back"
	CmdModeCommand   CommandID = "mode.command"
	CmdModeSearch    CommandID = "mode.search"
	CmdHelpToggle    CommandID = "help.toggle"
	CmdAppQuit       CommandID = "app.quit"
	CmdClusterCreate CommandID = "cluster.create"
)

// Mode is the current editing mode, mirroring vim's modal model.
type Mode int

const (
	ModeNormal Mode = iota
	ModeCommand
	ModeSearch
)

// Binding maps an ordered key sequence to a command within a mode.
type Binding struct {
	Keys []string  // e.g. {"g","g"} or {LeaderToken,"f","f"}
	ID   CommandID
	Desc string // which-key description
	Mode Mode
}

// ResolveStatus is the outcome of feeding one key to the resolver.
type ResolveStatus int

const (
	NoMatch ResolveStatus = iota
	Pending
	Resolved
)

// ResolveResult is returned by Resolver.Feed.
type ResolveResult struct {
	Status  ResolveStatus
	ID      CommandID // set when Status == Resolved
	Pending []Binding // candidates still matching (drives the which-key popup)
}

// Registry holds the leader key and the set of bindings.
type Registry struct {
	leader   string
	bindings []Binding
}

// NewRegistry creates a Registry with the given leader key (e.g. " ").
func NewRegistry(leader string) *Registry {
	return &Registry{leader: leader}
}

// Add registers a single binding, expanding any LeaderToken to the leader key.
func (r *Registry) Add(b Binding) {
	keys := make([]string, len(b.Keys))
	for i, k := range b.Keys {
		if k == LeaderToken {
			keys[i] = r.leader
		} else {
			keys[i] = k
		}
	}
	b.Keys = keys
	r.bindings = append(r.bindings, b)
}

// AddAll registers multiple bindings.
func (r *Registry) AddAll(bs []Binding) {
	for _, b := range bs {
		r.Add(b)
	}
}

// WhichKey returns the bindings in mode that continue the given prefix, i.e.
// bindings whose key sequence starts with prefix and is strictly longer.
func (r *Registry) WhichKey(prefix []string, mode Mode) []Binding {
	var out []Binding
	for _, b := range r.bindings {
		if b.Mode != mode {
			continue
		}
		if len(b.Keys) > len(prefix) && hasPrefix(b.Keys, prefix) {
			out = append(out, b)
		}
	}
	return out
}

func hasPrefix(keys, prefix []string) bool {
	if len(keys) < len(prefix) {
		return false
	}
	for i := range prefix {
		if keys[i] != prefix[i] {
			return false
		}
	}
	return true
}

// Resolver holds mutable pending state, the current mode, and a timeout clock.
type Resolver struct {
	reg     *Registry
	pending []string
	mode    Mode
	timeout time.Duration
	lastAt  time.Time
}

// NewResolver creates a Resolver bound to a Registry. timeout controls how long
// a partial multi-key sequence survives before the next key starts fresh.
func NewResolver(reg *Registry, timeout time.Duration) *Resolver {
	return &Resolver{reg: reg, timeout: timeout, mode: ModeNormal}
}

// Mode reports the resolver's current mode.
func (res *Resolver) Mode() Mode { return res.mode }

// Pending returns a copy of the keys accumulated so far.
func (res *Resolver) Pending() []string {
	out := make([]string, len(res.pending))
	copy(out, res.pending)
	return out
}

// Reset clears the pending key sequence (mode is preserved).
func (res *Resolver) Reset() { res.pending = nil }

// Feed processes one key at the given time and returns the resolution outcome.
func (res *Resolver) Feed(key string, now time.Time) ResolveResult {
	// Expire a stale partial sequence before considering the new key.
	if len(res.pending) > 0 && res.timeout > 0 && now.Sub(res.lastAt) > res.timeout {
		res.pending = nil
	}
	res.lastAt = now

	// esc is universal: cancel any pending sequence and return to normal mode.
	if key == "esc" {
		res.pending = nil
		res.mode = ModeNormal
		return ResolveResult{Status: Resolved, ID: CmdNavBack}
	}

	// In command/search mode, keystrokes are text input, not bindings.
	if res.mode != ModeNormal {
		return ResolveResult{Status: NoMatch}
	}

	res.pending = append(res.pending, key)

	var exact *Binding
	var prefixes []Binding
	for i := range res.reg.bindings {
		b := res.reg.bindings[i]
		if b.Mode != res.mode {
			continue
		}
		if len(b.Keys) == len(res.pending) && hasPrefix(b.Keys, res.pending) {
			bb := b
			exact = &bb
		} else if len(b.Keys) > len(res.pending) && hasPrefix(b.Keys, res.pending) {
			prefixes = append(prefixes, b)
		}
	}

	// A longer sequence is still possible: wait for more keys (which-key).
	if len(prefixes) > 0 {
		return ResolveResult{Status: Pending, Pending: prefixes}
	}

	if exact != nil {
		res.pending = nil
		res.applyModeTransition(exact.ID)
		return ResolveResult{Status: Resolved, ID: exact.ID}
	}

	// Nothing matches the accumulated sequence.
	res.pending = nil
	return ResolveResult{Status: NoMatch}
}

// applyModeTransition updates the resolver mode for mode-switching commands.
func (res *Resolver) applyModeTransition(id CommandID) {
	switch id {
	case CmdModeCommand:
		res.mode = ModeCommand
	case CmdModeSearch:
		res.mode = ModeSearch
	case CmdNavBack:
		res.mode = ModeNormal
	}
}

// DefaultBindings returns the Astronvim/nvim-style default binding set.
func DefaultBindings() []Binding {
	return []Binding{
		{Keys: []string{"j"}, ID: CmdNavDown, Desc: "down", Mode: ModeNormal},
		{Keys: []string{"k"}, ID: CmdNavUp, Desc: "up", Mode: ModeNormal},
		{Keys: []string{"g", "g"}, ID: CmdNavTop, Desc: "go to top", Mode: ModeNormal},
		{Keys: []string{"G"}, ID: CmdNavBottom, Desc: "go to bottom", Mode: ModeNormal},
		{Keys: []string{":"}, ID: CmdModeCommand, Desc: "command mode", Mode: ModeNormal},
		{Keys: []string{"/"}, ID: CmdModeSearch, Desc: "search", Mode: ModeNormal},
		{Keys: []string{"?"}, ID: CmdHelpToggle, Desc: "help", Mode: ModeNormal},
		{Keys: []string{LeaderToken, "f", "f"}, ID: CmdClusterCreate, Desc: "create cluster", Mode: ModeNormal},
		{Keys: []string{LeaderToken, "q"}, ID: CmdAppQuit, Desc: "quit", Mode: ModeNormal},
	}
}
