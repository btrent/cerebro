package tui

import "github.com/charmbracelet/lipgloss"

// styles holds every Lip Gloss style used by the UI, grouped in one place so the
// look can be tuned without hunting through the view code.
type styles struct {
	headerBar  lipgloss.Style
	headerName lipgloss.Style
	headerMeta lipgloss.Style

	statusReady   lipgloss.Style
	statusLoading lipgloss.Style
	statusError   lipgloss.Style

	userLabel      lipgloss.Style
	assistantLabel lipgloss.Style
	errorLabel     lipgloss.Style

	// Per-role left-border "gutters" — a colored vertical bar that spans the
	// full height of a message block.
	userGutter      lipgloss.Style
	assistantGutter lipgloss.Style
	errorGutter     lipgloss.Style

	systemText lipgloss.Style
	errorText  lipgloss.Style

	chip lipgloss.Style

	inputActive lipgloss.Style
	footer      lipgloss.Style
	hint        lipgloss.Style
}

func newStyles() styles {
	const (
		colUser      = lipgloss.Color("39")  // blue
		colAssistant = lipgloss.Color("78")  // green
		colError     = lipgloss.Color("203") // red
		colAccent    = lipgloss.Color("212") // pink
		colMuted     = lipgloss.Color("240")
		colSubtle    = lipgloss.Color("245")
		colBarBg     = lipgloss.Color("236")
		colBarFg     = lipgloss.Color("252")
	)

	// Left-only border using a half-block bar, colored per role. PaddingLeft adds
	// one space between the bar and the text.
	bar := lipgloss.Border{Left: "▌"}
	gutter := func(c lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().
			Border(bar, false, false, false, true).
			BorderForeground(c).
			PaddingLeft(1)
	}

	return styles{
		headerBar:  lipgloss.NewStyle().Background(colBarBg).Foreground(colBarFg).Padding(0, 1),
		headerName: lipgloss.NewStyle().Background(colBarBg).Foreground(colAccent).Bold(true),
		headerMeta: lipgloss.NewStyle().Background(colBarBg).Foreground(colBarFg),

		statusReady:   lipgloss.NewStyle().Foreground(colAssistant),
		statusLoading: lipgloss.NewStyle().Foreground(colAccent),
		statusError:   lipgloss.NewStyle().Foreground(colError),

		userLabel:      lipgloss.NewStyle().Foreground(colUser).Bold(true),
		assistantLabel: lipgloss.NewStyle().Foreground(colAssistant).Bold(true),
		errorLabel:     lipgloss.NewStyle().Foreground(colError).Bold(true),

		userGutter:      gutter(colUser),
		assistantGutter: gutter(colAssistant),
		errorGutter:     gutter(colError),

		// Body text intentionally has no explicit foreground so it uses the
		// terminal's default (readable on both light and dark themes).
		systemText: lipgloss.NewStyle().Foreground(colSubtle).Italic(true).PaddingLeft(2),
		errorText:  lipgloss.NewStyle().Foreground(colError),

		chip: lipgloss.NewStyle().Background(colMuted).Foreground(colBarFg).Padding(0, 1).MarginRight(1),

		inputActive: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colAccent).Padding(0, 1),
		footer:      lipgloss.NewStyle().Foreground(colMuted),
		hint:        lipgloss.NewStyle().Foreground(colMuted),
	}
}
