package llm

import (
	"fmt"
	"sort"
)

// ModelSpec is the backend-agnostic description of a configured model, derived
// from the user's config file. It is everything a Factory needs to build a
// Provider.
type ModelSpec struct {
	Provider       string // registered provider kind, e.g. "openai" or "ollama"
	Model          string // backend model identifier
	BaseURL        string // HTTP endpoint of the backend
	SupportsImages bool   // whether image input should be sent
	APIKey         string // optional; most local backends ignore it
}

// Factory builds a Provider from a ModelSpec.
type Factory func(ModelSpec) (Provider, error)

var registry = map[string]Factory{}

// Register makes a provider kind available to New. Clients call this from an
// init() function; the application imports them for their side effects. Calling
// Register twice with the same name panics, which surfaces programming errors at
// startup rather than silently shadowing a backend.
func Register(name string, f Factory) {
	if f == nil {
		panic("llm: Register called with nil factory for " + name)
	}
	if _, dup := registry[name]; dup {
		panic("llm: Register called twice for provider " + name)
	}
	registry[name] = f
}

// New builds a Provider for the given spec, returning a helpful error if the
// provider kind has not been registered.
func New(spec ModelSpec) (Provider, error) {
	f, ok := registry[spec.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q (registered: %v)", spec.Provider, Registered())
	}
	return f(spec)
}

// Registered returns the sorted list of registered provider kinds, useful for
// error messages and diagnostics.
func Registered() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
