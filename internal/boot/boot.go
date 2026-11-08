// Package boot runs terminal setup before Bubble Tea's own package init.
//
// Bubble Tea v1.3 calls lipgloss.HasDarkBackground() from init, which queries
// the terminal with OSC 11 and waits up to 5s per read when nothing answers
// (CI, dumb ptys, some terminals). file-cli renders with fixed colors, so the
// answer is irrelevant: pin it and skip the query.
package boot

import "github.com/charmbracelet/lipgloss"

func init() {
	lipgloss.DefaultRenderer().SetHasDarkBackground(true)
}
