// Package history persists conversations as JSON under the data directory. It
// stores the current session and supports saving/loading named conversations.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cerebro/internal/paths"
)

// Turn is a single message in a stored conversation. Images holds source file
// paths (not bytes) so transcripts stay small.
type Turn struct {
	Role   string    `json:"role"`
	Text   string    `json:"text"`
	Images []string  `json:"images,omitempty"`
	Time   time.Time `json:"time"`
}

// Conversation is an ordered list of turns with metadata.
type Conversation struct {
	Title   string    `json:"title"`
	Model   string    `json:"model"`
	Created time.Time `json:"created"`
	Turns   []Turn    `json:"turns"`

	file string // resolved path, set when saved/loaded
}

// NewSession creates an empty conversation stamped with the current time.
func NewSession(model string) *Conversation {
	now := time.Now()
	return &Conversation{
		Title:   "session-" + now.Format("2006-01-02-150405"),
		Model:   model,
		Created: now,
	}
}

// Append adds a turn to the conversation.
func (c *Conversation) Append(role, text string, imagePaths []string) {
	c.Turns = append(c.Turns, Turn{
		Role:   role,
		Text:   text,
		Images: imagePaths,
		Time:   time.Now(),
	})
}

// Clear removes all turns (used by /clear).
func (c *Conversation) Clear() { c.Turns = nil }

// Save writes the conversation to the history directory. The current session is
// saved under its Title; SaveAs can override the name for explicit saves.
func (c *Conversation) Save() error {
	if c.file == "" {
		c.file = filepath.Join(paths.HistoryDir(), sanitize(c.Title)+".json")
	}
	return c.writeTo(c.file)
}

// SaveAs persists the conversation under an explicit name and adopts that file
// as its location.
func (c *Conversation) SaveAs(name string) error {
	c.Title = name
	c.file = filepath.Join(paths.HistoryDir(), sanitize(name)+".json")
	return c.writeTo(c.file)
}

func (c *Conversation) writeTo(path string) error {
	if err := paths.EnsureDir(paths.HistoryDir()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load reads a conversation from a path.
func Load(path string) (*Conversation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Conversation
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid conversation %s: %w", path, err)
	}
	c.file = path
	return &c, nil
}

// List returns the saved conversation file paths, newest first.
func List() ([]string, error) {
	dir := paths.HistoryDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files, nil
}

// sanitize makes a string safe for use as a filename.
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "untitled"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-")
	return replacer.Replace(s)
}
