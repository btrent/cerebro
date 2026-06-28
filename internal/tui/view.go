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

	headerH := 1
	footerH := 2
	if m.showFullHelp {
		footerH = 4
	}
	chipsH := 0
	if len(m.currentAttachments()) > 0 {
		chipsH = 1
	}

	m.input.SetWidth(m.width - 4)
	m.input.SetHeight(3)
	inputH := 3 + 2 + chipsH // textarea + border + chips line

	vpH := m.height - headerH - footerH - inputH
	if vpH < 1 {
		vpH = 1
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpH

	m.help.Width = m.width
	m.editor.SetWidth(m.width - 4)
	m.editor.SetHeight(m.height - headerH - footerH - 4)

	m.rebuildRenderer()
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
	body := m.viewport.View()

	var inputBlock string
	if m.mode == modeHeaderEdit {
		inputBlock = m.renderEditor()
	} else {
		inputBlock = m.renderInput()
	}

	footer := m.renderFooter()
	return strings.Join([]string{header, body, inputBlock, footer}, "\n")
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
	return m.st.headerBar.Width(m.width).Render(truncate(line, m.width))
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
