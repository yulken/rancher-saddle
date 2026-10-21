package views

import (
	"os"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
)

// copyToClipboard copies text to the system clipboard using an OSC52 escape
// sequence. OSC52 is interpreted by the terminal emulator itself, so it works
// locally and over SSH without needing xclip/pbcopy/wl-copy. The sequence is
// written to stderr (the controlling terminal) to avoid interleaving with
// Bubbletea's stdout frame buffer. Terminals that don't support OSC52 simply
// ignore it.
func copyToClipboard(text string) {
	seq := osc52.New(text)
	// Inside tmux the sequence must be wrapped so tmux passes it through.
	if os.Getenv("TMUX") != "" {
		seq = seq.Tmux()
	}
	_, _ = seq.WriteTo(os.Stderr)
}
