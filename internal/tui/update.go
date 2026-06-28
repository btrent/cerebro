package tui

import (
	"context"
	"regexp"
	"strings"

	"cerebro/internal/images"
	"cerebro/internal/llm"
	"cerebro/internal/prompts"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// --- async messages -----------------------------------------------------------

type backendReadyMsg struct{ err error }

type streamChunkMsg struct{ chunk llm.Chunk }

// ensureBackendCmd brings the backend up (launching it if configured). It blocks
// in its own goroutine so the UI stays responsive while the model loads.
func ensureBackendCmd(mgr *llm.Manager) tea.Cmd {
	return func() tea.Msg {
		if mgr == nil {
			return backendReadyMsg{}
		}
		err := mgr.EnsureRunning(context.Background(), nil)
		return backendReadyMsg{err: err}
	}
}

// waitChunkCmd reads the next chunk from the stream channel.
func waitChunkCmd(ch <-chan llm.Chunk) tea.Cmd {
	return func() tea.Msg {
		c, ok := <-ch
		if !ok {
			return streamChunkMsg{chunk: llm.Chunk{Done: true}}
		}
		return streamChunkMsg{chunk: c}
	}
}

// Update is the central event handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.applyLayout()
		m.refreshViewport()
		return m, nil

	case backendReadyMsg:
		if msg.err != nil {
			m.backendErr = msg.err
			m.backendStatus = "Backend unavailable"
			m.addError("Backend unavailable: " + msg.err.Error())
			m.addSystem("Fix config at " + m.deps.Config.Path() + " or run `cerebro --setup`, then restart.")
		} else {
			m.backendReady = true
			m.backendStatus = "Ready"
			m.addSystem("Backend ready. Send a message to begin.")
		}
		m.refreshViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.streaming || !m.backendReady && m.backendErr == nil {
			return m, cmd
		}
		return m, nil

	case streamChunkMsg:
		return m.handleChunk(msg.chunk)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward anything else to the focused input component.
	return m, m.forwardToInput(msg)
}

func (m Model) forwardToInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if m.mode == modeHeaderEdit {
		m.editor, cmd = m.editor.Update(msg)
	} else {
		m.input, cmd = m.input.Update(msg)
	}
	return cmd
}

// handleKey routes key presses based on the current mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit works in every mode.
	if key := msg.String(); key == "ctrl+c" || key == "ctrl+d" {
		m.quitting = true
		if m.streamCancel != nil {
			m.streamCancel()
		}
		return m, tea.Quit
	}

	if m.mode == modeHeaderEdit {
		return m.handleEditKey(msg)
	}

	switch msg.String() {
	case "esc":
		if m.streaming && m.streamCancel != nil {
			m.streamCancel()
			m.streaming = false
			m.finalizeAssistant(" _(stopped)_")
			m.refreshViewport()
		}
		return m, nil

	case "ctrl+l":
		return m.clearConversation(), nil

	case "ctrl+h":
		m.showFullHelp = !m.showFullHelp
		m.help.ShowAll = m.showFullHelp
		m.applyLayout()
		m.refreshViewport()
		return m, nil

	case "enter":
		if m.streaming {
			return m, nil // ignore submit while streaming
		}
		return m.submit()
	}

	// Typed runes and pastes: strip terminal query-response noise (OSC color /
	// cursor reports some terminals emit at startup) so it never lands in the
	// input, and collapse large pastes.
	if msg.Type == tea.KeyRunes || msg.Paste {
		clean, pure := cleanInput(msg)
		if pure {
			return m, nil // pure terminal report → drop
		}
		if msg.Paste {
			m.input.InsertString(m.pasteBuf.Collapse(clean))
			return m, nil
		}
		if clean != string(msg.Runes) {
			m.input.InsertString(clean)
			return m, nil
		}
		// clean == raw: ordinary typing, fall through to the textarea.
	}

	// Everything else (typing, arrows, ctrl+j newline) goes to the textarea,
	// while page-up/down scroll the transcript.
	switch msg.String() {
	case "pgup", "pgdown":
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.follow = m.viewport.AtBottom() // stop following once scrolled up
		return m, cmd
	case "home":
		m.viewport.GotoTop()
		m.follow = false
		return m, nil
	case "end":
		m.viewport.GotoBottom()
		m.follow = true
		return m, nil
	case "ctrl+up":
		m.viewport.ScrollUp(1)
		m.follow = m.viewport.AtBottom()
		return m, nil
	case "ctrl+down":
		m.viewport.ScrollDown(1)
		m.follow = m.viewport.AtBottom()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// terminalReportRe matches the terminal query responses that can leak into input
// at startup: OSC color reports (]11;rgb:…) and CSI cursor-position reports
// ([row;colR). The leading ESC byte is consumed by the input parser before we
// see these, so it is not part of the pattern.
var terminalReportRe = regexp.MustCompile(`\][0-9]+;rgb:[0-9a-fA-F/]+\x07?|\[[0-9;]*R`)

func stripTerminalReports(s string) string {
	return terminalReportRe.ReplaceAllString(s, "")
}

// submit handles an Enter press in chat mode: slash commands, exit words, or a
// model prompt.
func (m Model) submit() (tea.Model, tea.Cmd) {
	raw := m.input.Value()
	expanded := m.pasteBuf.Expand(raw)
	trimmed := strings.TrimSpace(expanded)

	if trimmed == "" {
		return m, nil // empty submission ignored
	}
	// Any submission means the user wants to see fresh output: re-pin to bottom.
	m.follow = true
	if eq(trimmed, "exit") || eq(trimmed, "quit") {
		m.quitting = true
		return m, tea.Quit
	}
	if strings.HasPrefix(trimmed, "/") {
		newM, cmd := m.runCommand(trimmed)
		newM.input.Reset()
		newM.pasteBuf.Reset()
		newM.refreshViewport()
		return newM, cmd
	}

	if !m.backendReady {
		if m.backendErr != nil {
			m.addError("Cannot send: backend is unavailable. See messages above.")
		} else {
			m.addSystem("Backend is still loading — please wait a moment.")
		}
		m.refreshViewport()
		return m, nil
	}

	return m.sendPrompt(raw, expanded)
}

// sendPrompt assembles context, attaches images, and starts streaming.
func (m Model) sendPrompt(displayText, modelText string) (tea.Model, tea.Cmd) {
	// Detect and load image attachments.
	var attached []llm.Image
	var attachedPaths []string
	for _, p := range images.DetectPaths(modelText) {
		img, err := images.Load(p)
		if err != nil {
			m.addError(err.Error())
			continue
		}
		attached = append(attached, img)
		attachedPaths = append(attachedPaths, p)
		// Remove the bare path token from the text sent to the model.
		modelText = strings.ReplaceAll(modelText, p, "")
		displayText = strings.ReplaceAll(displayText, p, "")
	}

	// Capability check: drop images for text-only backends, with a warning.
	if len(attached) > 0 && !m.deps.Provider.SupportsImages() {
		m.addError("Active model/backend does not support images — sending text only.")
		attached = nil
		attachedPaths = nil
	}

	displayText = strings.TrimSpace(displayText)
	modelText = strings.TrimSpace(modelText)
	if modelText == "" && len(attached) == 0 {
		return m, nil
	}

	// Build conversation history from prior turns BEFORE adding this one.
	hist := m.buildHistory()

	m.messages = append(m.messages, chatMessage{
		kind: kindUser, content: displayText, modelText: modelText, images: attachedPaths,
	})
	m.deps.Session.Append("user", modelText, attachedPaths)

	user := llm.Message{Role: llm.RoleUser, Text: modelText, Images: attached}
	headerBody := ""
	if ap, ok := m.deps.Prompts.ActivePrompt(); ok {
		headerBody = ap.Body
	}
	assembled := prompts.Assemble(headerBody, hist, user)

	m.assistantIdx = m.addAssistantPlaceholder()
	m.input.Reset()
	m.pasteBuf.Reset()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := m.deps.Provider.Stream(ctx, llm.ChatRequest{Messages: assembled, Stream: true})
	if err != nil {
		cancel()
		m.messages[m.assistantIdx] = chatMessage{kind: kindError, content: "Request failed: " + err.Error()}
		m.refreshViewport()
		return m, nil
	}
	m.streaming = true
	m.streamCh = ch
	m.streamCancel = cancel
	m.refreshViewport()
	return m, tea.Batch(waitChunkCmd(ch), m.spinner.Tick)
}

// handleChunk consumes one streamed chunk.
func (m Model) handleChunk(c llm.Chunk) (tea.Model, tea.Cmd) {
	if !m.streaming {
		return m, nil // stale chunk after cancel
	}
	switch {
	case c.Err != nil:
		m.streaming = false
		if m.streamCancel != nil {
			m.streamCancel()
		}
		if strings.TrimSpace(m.messages[m.assistantIdx].content) == "" {
			m.messages[m.assistantIdx] = chatMessage{kind: kindError, content: "Error: " + c.Err.Error()}
		} else {
			m.messages[m.assistantIdx].content += "\n\n_(error: " + c.Err.Error() + ")_"
		}
		m.persistAssistant()
		m.refreshViewport()
		return m, nil

	case c.Done:
		m.streaming = false
		if m.streamCancel != nil {
			m.streamCancel()
		}
		m.finalizeAssistant("")
		m.persistAssistant()
		m.refreshViewport()
		return m, nil

	default:
		m.messages[m.assistantIdx].content += c.Delta
		m.refreshViewport()
		return m, waitChunkCmd(m.streamCh)
	}
}

// finalizeAssistant renders the completed assistant message as Markdown (cached)
// and optionally appends a suffix note.
func (m *Model) finalizeAssistant(suffix string) {
	if m.assistantIdx < 0 || m.assistantIdx >= len(m.messages) {
		return
	}
	msg := &m.messages[m.assistantIdx]
	if suffix != "" {
		msg.content += suffix
	}
	msg.rendered = m.renderMarkdown(msg.content)
}

func (m *Model) persistAssistant() {
	if m.assistantIdx < 0 || m.assistantIdx >= len(m.messages) {
		return
	}
	m.deps.Session.Append("assistant", m.messages[m.assistantIdx].content, nil)
	_ = m.deps.Session.Save() // best-effort
}

// buildHistory converts prior user/assistant transcript entries into llm
// messages for context (errors and system notices are excluded).
func (m *Model) buildHistory() []llm.Message {
	var out []llm.Message
	for _, cm := range m.messages {
		switch cm.kind {
		case kindUser:
			out = append(out, llm.Message{Role: llm.RoleUser, Text: textOf(cm)})
		case kindAssistant:
			if strings.TrimSpace(cm.content) != "" {
				out = append(out, llm.Message{Role: llm.RoleAssistant, Text: cm.content})
			}
		}
	}
	return out
}

func (m Model) clearConversation() Model {
	m.messages = nil
	m.deps.Session.Clear()
	m.addSystem("Conversation cleared.")
	m.refreshViewport()
	return m
}

// renderMarkdown renders s with Glamour, falling back to raw text on error or if
// the renderer isn't ready yet.
func (m *Model) renderMarkdown(s string) string {
	if m.renderer == nil {
		return s
	}
	out, err := m.renderer.Render(s)
	if err != nil {
		return s
	}
	return strings.TrimRight(out, "\n")
}

func (m *Model) rebuildRenderer() {
	// Leave room for the gutter (border + padding) and Glamour's own margin.
	width := m.width - 6
	if width < 20 {
		width = 20
	}
	if m.renderer != nil && m.rendererWidth == width {
		return
	}
	// Use the "notty" style: no ANSI colors, so assistant text renders in the
	// terminal's default foreground (readable on any theme) instead of a dim
	// grey. This also avoids the terminal background query that WithAutoStyle
	// performs (which could leak into input).
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("notty"), glamour.WithWordWrap(width))
	if err == nil {
		m.renderer = r
		m.rendererWidth = width
	}
}

// cleanInput strips terminal report sequences from a runes/paste key message,
// returning the printable remainder and whether the message was pure noise.
func cleanInput(msg tea.KeyMsg) (text string, pureReport bool) {
	raw := string(msg.Runes)
	clean := stripTerminalReports(raw)
	if raw != "" && strings.TrimSpace(clean) == "" {
		return "", true
	}
	return clean, false
}

func eq(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), b) }

func textOf(cm chatMessage) string {
	if cm.modelText != "" {
		return cm.modelText
	}
	return cm.content
}
