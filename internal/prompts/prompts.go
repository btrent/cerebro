// Package prompts manages header prompts: reusable instruction blocks that are
// prepended (as a system message) to every conversation until changed or
// disabled. The store persists to header_prompts.yaml and supports full CRUD
// plus selecting/disabling the active prompt.
package prompts

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"cerebro/internal/paths"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned when a named header prompt does not exist.
var ErrNotFound = errors.New("header prompt not found")

// HeaderPrompt is a single named, reusable instruction block.
type HeaderPrompt struct {
	Name string `yaml:"name"`
	Body string `yaml:"body"`
}

// Store holds all header prompts and which one (if any) is active.
type Store struct {
	Active  string         `yaml:"active"` // "" means none/disabled
	Prompts []HeaderPrompt `yaml:"prompts"`

	path string `yaml:"-"`
}

// Load reads the store from path. A missing file yields an empty store (with no
// active prompt). A malformed file returns an error.
func Load(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("invalid header prompts %s: %w", path, err)
	}
	s.path = path
	// If the active prompt was deleted out-of-band, disable it.
	if s.Active != "" && s.find(s.Active) < 0 {
		s.Active = ""
	}
	return s, nil
}

// LoadDefault loads from the standard header_prompts.yaml location.
func LoadDefault() (*Store, error) { return Load(paths.HeaderPromptsFile()) }

// Save persists the store to its source path.
func (s *Store) Save() error {
	path := s.path
	if path == "" {
		path = paths.HeaderPromptsFile()
	}
	if err := paths.EnsureDir(paths.ConfigDir()); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *Store) find(name string) int {
	for i, p := range s.Prompts {
		if strings.EqualFold(p.Name, name) {
			return i
		}
	}
	return -1
}

// Names returns prompt names sorted alphabetically.
func (s *Store) Names() []string {
	names := make([]string, len(s.Prompts))
	for i, p := range s.Prompts {
		names[i] = p.Name
	}
	sort.Strings(names)
	return names
}

// Get returns the prompt with the given name.
func (s *Store) Get(name string) (HeaderPrompt, error) {
	if i := s.find(name); i >= 0 {
		return s.Prompts[i], nil
	}
	return HeaderPrompt{}, ErrNotFound
}

// Create adds a new prompt. It errors if the name is empty or already exists.
func (s *Store) Create(name, body string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("header prompt name cannot be empty")
	}
	if s.find(name) >= 0 {
		return fmt.Errorf("header prompt %q already exists", name)
	}
	s.Prompts = append(s.Prompts, HeaderPrompt{Name: name, Body: body})
	return nil
}

// Update replaces the body of an existing prompt.
func (s *Store) Update(name, body string) error {
	i := s.find(name)
	if i < 0 {
		return ErrNotFound
	}
	s.Prompts[i].Body = body
	return nil
}

// Upsert creates the prompt or updates it if it already exists.
func (s *Store) Upsert(name, body string) error {
	if s.find(name) >= 0 {
		return s.Update(name, body)
	}
	return s.Create(name, body)
}

// Delete removes a prompt; if it was active, the active selection is cleared.
func (s *Store) Delete(name string) error {
	i := s.find(name)
	if i < 0 {
		return ErrNotFound
	}
	if strings.EqualFold(s.Active, s.Prompts[i].Name) {
		s.Active = ""
	}
	s.Prompts = append(s.Prompts[:i], s.Prompts[i+1:]...)
	return nil
}

// Use sets the active prompt by name.
func (s *Store) Use(name string) error {
	i := s.find(name)
	if i < 0 {
		return ErrNotFound
	}
	s.Active = s.Prompts[i].Name // normalize to stored casing
	return nil
}

// None disables the active prompt.
func (s *Store) None() { s.Active = "" }

// ActivePrompt returns the active prompt and true, or a zero value and false if
// none is active.
func (s *Store) ActivePrompt() (HeaderPrompt, bool) {
	if s.Active == "" {
		return HeaderPrompt{}, false
	}
	if i := s.find(s.Active); i >= 0 {
		return s.Prompts[i], true
	}
	return HeaderPrompt{}, false
}

// ActiveName returns the active prompt name, or "none".
func (s *Store) ActiveName() string {
	if s.Active == "" {
		return "none"
	}
	return s.Active
}
