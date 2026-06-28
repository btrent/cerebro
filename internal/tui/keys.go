package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines the application-level key bindings. Editing/navigation keys are
// handled by the embedded textarea and viewport components.
type keyMap struct {
	Submit  key.Binding
	Newline key.Binding
	Cancel  key.Binding
	Quit    key.Binding
	Clear   key.Binding
	Help    key.Binding
	ScrollU key.Binding
	ScrollD key.Binding
	LineU   key.Binding
	LineD   key.Binding
	Top     key.Binding
	Bottom  key.Binding
	Scroll  key.Binding // display-only combined hint for the footer
}

func defaultKeyMap() keyMap {
	return keyMap{
		Submit:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		Newline: key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "newline")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop")),
		Quit:    key.NewBinding(key.WithKeys("ctrl+c", "ctrl+d"), key.WithHelp("ctrl+c", "quit")),
		Clear:   key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "clear")),
		Help:    key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "help")),
		ScrollU: key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
		ScrollD: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "scroll down")),
		LineU:   key.NewBinding(key.WithKeys("ctrl+up"), key.WithHelp("ctrl+↑", "line up")),
		LineD:   key.NewBinding(key.WithKeys("ctrl+down"), key.WithHelp("ctrl+↓", "line down")),
		Top:     key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		Bottom:  key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
		Scroll:  key.NewBinding(key.WithKeys("pgup", "pgdown", "home", "end"), key.WithHelp("pgup/pgdn", "scroll")),
	}
}

// ShortHelp implements help.KeyMap for the compact footer.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Newline, k.Scroll, k.Clear, k.Quit}
}

// FullHelp implements help.KeyMap for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Submit, k.Newline, k.Cancel},
		{k.ScrollU, k.ScrollD, k.LineU, k.LineD},
		{k.Top, k.Bottom, k.Clear},
		{k.Help, k.Quit},
	}
}
