package tui

import (
	"fmt"
	"strings"

	"cerebro/internal/images"

	"github.com/charmbracelet/lipgloss"
)

const (
	minWidth  = 40
	minHeight = 12
)

// applyLayout recomputes component sizes from the current terminal dimensions.
func (m *Model) applyLayout() {
	m.tooSmall = m.width < minWidth || m.height < minHeight
	if m.tooSmall {
		return
	}

	m.input.SetWidth(m.width - 4)
	m.input.SetHeight(3)
	m.help.Width = m.width

	// Bounded editor height (it scrolls internally for longer prompts), leaving
	// room to still see some of the conversation behind it.
	edH := m.height - 12
	if edH < 3 {
		edH = 3
	} else if edH > 16 {
		edH = 16
	}
	m.editor.SetWidth(m.width - 4)
	m.editor.SetHeight(edH)

	m.rebuildRenderer()

	m.viewport.Width = m.width
	m.viewport.Height = m.bodyHeight()
}

// lowerView is the block shown beneath the transcript: the prompt input, or the
// header editor when editing.
func (m *Model) lowerView() string {
	if m.mode == modeHeaderEdit {
		return m.renderEditor()
	}
	return m.renderInput()
}

// bodyHeight is the height available to the transcript viewport: the terminal
// height minus the actual rendered heights of the header, lower block, and
// footer. Measuring real output keeps the total at exactly the terminal height
// regardless of wrapping in any chrome element.
func (m *Model) bodyHeight() int {
	h := m.height - lipgloss.Height(m.renderHeader()) - lipgloss.Height(m.lowerView()) - lipgloss.Height(m.renderFooter())
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) contentWidth() int {
	if m.width < 4 {
		return 60
	}
	return m.width - 2
}

func (m *Model) currentAttachments() []string {
	return images.DetectPaths(m.pasteBuf.Expand(m.input.Value()))
}

// refreshViewport rebuilds the transcript content and pins to the bottom.
func (m *Model) refreshViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.renderTranscript())
	// Only pin to the bottom when following; otherwise preserve the user's
	// scroll position so they can read earlier history while output streams in.
	if m.follow {
		m.viewport.GotoBottom()
	}
}

func (m *Model) renderTranscript() string {
	var b strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.renderMessage(msg))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) renderMessage(msg chatMessage) string {
	switch msg.kind {
	case kindUser:
		block := m.st.userLabel.Render("You") + "\n" + msg.content
		if len(msg.images) > 0 {
			block += "\n" + m.renderChips(msg.images)
		}
		return m.st.userGutter.Render(block)

	case kindAssistant:
		content := msg.rendered
		if content == "" {
			// streaming or not yet rendered → raw text (default terminal color)
			content = msg.content
			if m.streaming && strings.TrimSpace(msg.content) == "" {
				content = m.st.hint.Render(m.spinner.View() + " thinking…")
			}
		}
		return m.st.assistantGutter.Render(m.st.assistantLabel.Render("cerebro") + "\n" + content)

	case kindError:
		block := m.st.errorLabel.Render("error") + "\n" + m.st.errorText.Render(msg.content)
		return m.st.errorGutter.Render(block)

	default: // system
		return m.st.systemText.Render(msg.content)
	}
}

func (m *Model) renderChips(paths []string) string {
	var chips []string
	for _, p := range paths {
		chips = append(chips, m.st.chip.Render("🖼 "+shortPath(p)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, chips...)
}

// View renders the whole UI.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye.\n"
	}
	if !m.ready {
		return "Starting cerebro…\n"
	}
	if m.tooSmall {
		return fmt.Sprintf("Terminal too small (%dx%d).\nResize to at least %dx%d.\n",
			m.width, m.height, minWidth, minHeight)
	}

	header := m.renderHeader()
	lower := m.lowerView()
	footer := m.renderFooter()

	// Size the viewport to exactly the space left by the actual chrome so the
	// total is always the terminal height (no overflow into the input area).
	vpH := m.height - lipgloss.Height(header) - lipgloss.Height(lower) - lipgloss.Height(footer)
	if vpH < 1 {
		vpH = 1
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpH
	if m.follow {
		m.viewport.GotoBottom()
	}
	body := m.viewport.View()

	return strings.Join([]string{header, body, lower, footer}, "\n")
}

func (m *Model) renderHeader() string {
	name := m.st.headerName.Render("cerebro")

	modelName := m.deps.Config.ActiveModel
	if mc, ok := m.deps.Config.ActiveModelConfig(); ok && mc.DisplayName != "" {
		modelName = mc.DisplayName
	}

	meta := fmt.Sprintf("  Model: %s  •  Header: %s  •  %s",
		modelName, m.deps.Prompts.ActiveName(), m.statusText())

	line := name + m.st.headerMeta.Render(meta)
	// Account for the bar's horizontal padding (2) and clamp to a single line so
	// a long status string can never wrap the header.
	return m.st.headerBar.Width(m.width).MaxHeight(1).Render(truncate(line, m.width-2))
}

func (m *Model) statusText() string {
	switch {
	case m.backendErr != nil:
		return m.st.statusError.Render("● offline")
	case m.streaming:
		return m.st.statusLoading.Render(m.spinner.View() + " generating")
	case m.backendReady:
		return m.st.statusReady.Render("● ready")
	default:
		return m.st.statusLoading.Render(m.spinner.View() + " loading")
	}
}

func (m *Model) renderInput() string {
	var parts []string
	if atts := m.currentAttachments(); len(atts) > 0 {
		parts = append(parts, m.renderChips(atts))
	}
	box := m.st.inputActive
	parts = append(parts, box.Width(m.width-2).Render(m.input.View()))
	return strings.Join(parts, "\n")
}

func (m *Model) renderEditor() string {
	title := m.st.headerName.Render(fmt.Sprintf(" editing header: %s ", m.editName))
	return title + "\n" + m.st.inputActive.Width(m.width-2).Render(m.editor.View())
}

func (m *Model) renderFooter() string {
	if m.mode == modeHeaderEdit {
		return m.st.footer.Render("enter save  •  ctrl+j newline  •  esc cancel")
	}
	hint := ""
	switch {
	case m.backendErr != nil:
		hint = m.st.statusError.Render("backend offline — see messages • ")
	case !m.backendReady:
		hint = m.st.statusLoading.Render(m.spinner.View() + " loading model… • ")
	case m.streaming:
		hint = m.st.statusLoading.Render(m.spinner.View() + " generating (esc to stop) • ")
	}
	return hint + m.st.footer.Render(m.help.View(m.keys))
}

// --- small string helpers -----------------------------------------------------

func truncate(s string, w int) string {
	if w <= 0 || lipgloss.Width(s) <= w {
		return s
	}
	// Trim by runes until it fits (accounts for wide chars conservatively).
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > w {
		r = r[:len(r)-1]
	}
	return string(r)
}

func shortPath(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}
