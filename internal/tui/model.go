package tui

import (
	"context"

	"cerebro/internal/config"
	"cerebro/internal/history"
	"cerebro/internal/llm"
	"cerebro/internal/paste"
	"cerebro/internal/prompts"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// Deps are the collaborators the TUI needs, constructed by the app layer.
type Deps struct {
	Config   *config.Config
	Prompts  *prompts.Store
	Session  *history.Conversation
	Manager  *llm.Manager
	Provider llm.Provider
}

type msgKind int

const (
	kindUser msgKind = iota
	kindAssistant
	kindSystem
	kindError
)

// chatMessage is one rendered entry in the transcript.
type chatMessage struct {
	kind      msgKind
	content   string   // text shown in the transcript
	modelText string   // text sent to the model (expanded pastes); falls back to content
	rendered  string   // cached Markdown render for completed assistant messages
	images    []string // attached image paths (user messages)
}

type uiMode int

const (
	modeChat uiMode = iota
	modeHeaderEdit
)

// Model is the root Bubble Tea model.
type Model struct {
	deps  Deps
	keys  keyMap
	st    styles
	ready bool // received first WindowSizeMsg

	width, height int
	tooSmall      bool

	viewport      viewport.Model
	input         textarea.Model
	editor        textarea.Model // header-prompt editor (modeHeaderEdit)
	spinner       spinner.Model
	help          help.Model
	renderer      *glamour.TermRenderer
	rendererWidth int // width the current renderer was built for

	messages []chatMessage
	pasteBuf *paste.Buffer

	// backend lifecycle
	backendReady  bool
	backendErr    error
	backendStatus string

	// streaming state
	streaming    bool
	streamCh     <-chan llm.Chunk
	streamCancel context.CancelFunc
	assistantIdx int

	// header edit mode
	mode      uiMode
	editName  string
	editIsNew bool

	follow       bool // keep the transcript pinned to the bottom as new output arrives
	showFullHelp bool
	quitting     bool
}

// New builds the root model from its dependencies.
func New(deps Deps) Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message (Enter to send, Ctrl+J for newline, /help for commands)"
	ta.Prompt = "" // no per-line gutter
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.MaxHeight = 8
	// Enter is reserved for submit; Ctrl+J inserts a newline.
	ta.KeyMap.InsertNewline.SetEnabled(true)
	ta.KeyMap.InsertNewline.SetKeys("ctrl+j")
	ta.Focus()

	ed := textarea.New()
	ed.Placeholder = "Header prompt body…"
	ed.Prompt = ""
	ed.ShowLineNumbers = false
	ed.CharLimit = 0
	ed.MaxHeight = 20
	// Enter saves the header; Ctrl+J inserts a newline (handled in handleEditKey).
	ed.KeyMap.InsertNewline.SetEnabled(true)
	ed.KeyMap.InsertNewline.SetKeys("ctrl+j")

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := Model{
		deps:          deps,
		keys:          defaultKeyMap(),
		st:            newStyles(),
		viewport:      viewport.New(80, 20), // resized on first WindowSizeMsg
		input:         ta,
		editor:        ed,
		spinner:       sp,
		help:          help.New(),
		pasteBuf:      paste.New(),
		backendStatus: "Starting backend…",
		mode:          modeChat,
		follow:        true,
	}
	m.addSystem("Welcome to cerebro. Type /help for commands.")
	return m
}

// Init starts the spinner and kicks off backend startup.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, ensureBackendCmd(m.deps.Manager))
}

// --- message helpers ----------------------------------------------------------

func (m *Model) addUser(content string, images []string) {
	m.messages = append(m.messages, chatMessage{kind: kindUser, content: content, images: images})
}

func (m *Model) addSystem(content string) {
	m.messages = append(m.messages, chatMessage{kind: kindSystem, content: content})
}

func (m *Model) addError(content string) {
	m.messages = append(m.messages, chatMessage{kind: kindError, content: content})
}

// addAssistantPlaceholder appends an empty assistant message and returns its index.
func (m *Model) addAssistantPlaceholder() int {
	m.messages = append(m.messages, chatMessage{kind: kindAssistant})
	return len(m.messages) - 1
}
