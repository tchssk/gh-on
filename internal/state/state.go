// Package state keeps what gh-on has seen and handled between runs.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	ghconfig "github.com/cli/go-gh/v2/pkg/config"
)

// Dir returns the directory for the state file and the lock file.
func Dir() string {
	return filepath.Join(ghconfig.StateDir(), "gh-on")
}

// Path returns the location of the state file.
func Path() string { return filepath.Join(Dir(), "state.json") }

type State struct {
	ETag  string    `json:"etag,omitempty"`
	Since time.Time `json:"since,omitzero"` // the latest updated_at seen

	LastRun   time.Time `json:"last_run,omitzero"`
	LastError string    `json:"last_error,omitempty"`

	Rules map[string]*Rule `json:"rules,omitempty"`
}

type Rule struct {
	// Pending means a notification matched but the targets have not been checked yet.
	Pending bool `json:"pending,omitempty"`
	// Done maps the keys of handled targets to when the command succeeded.
	Done map[string]time.Time `json:"done,omitempty"`
}

// Rule returns the state of the named rule, creating it if needed.
func (s *State) Rule(name string) *Rule {
	if s.Rules == nil {
		s.Rules = map[string]*Rule{}
	}
	r := s.Rules[name]
	if r == nil {
		r = &Rule{}
		s.Rules[name] = r
	}
	if r.Done == nil {
		r.Done = map[string]time.Time{}
	}
	return r
}

// Retain drops the state of rules that are not in names.
func (s *State) Retain(names []string) {
	keep := map[string]bool{}
	for _, n := range names {
		keep[n] = true
	}
	for n := range s.Rules {
		if !keep[n] {
			delete(s.Rules, n)
		}
	}
}

// Load reads the state file. A missing file is an empty state.
func Load(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Save writes the state file atomically.
func (s *State) Save(path string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
