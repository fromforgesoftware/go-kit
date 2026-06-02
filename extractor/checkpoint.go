package extractor

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
)

// Checkpoint is the persistent ledger of completed units. Backends:
// MemoryCheckpoint (tests), FileCheckpoint (JSON sidecar), DBCheckpoint
// (planned). The interface stays minimal so any KV-shaped backend
// fits.
type Checkpoint interface {
	Done(unit string) (bool, error)
	Mark(unit string) error
	Reset(unit string) error
	All() ([]string, error)
}

// MemoryCheckpoint is an in-process Checkpoint suited for tests.
type MemoryCheckpoint struct {
	mu   sync.Mutex
	done map[string]struct{}
}

// NewMemoryCheckpoint constructs an empty MemoryCheckpoint.
func NewMemoryCheckpoint() *MemoryCheckpoint {
	return &MemoryCheckpoint{done: map[string]struct{}{}}
}

// Done implements Checkpoint.
func (m *MemoryCheckpoint) Done(unit string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.done[unit]
	return ok, nil
}

// Mark implements Checkpoint.
func (m *MemoryCheckpoint) Mark(unit string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.done[unit] = struct{}{}
	return nil
}

// Reset implements Checkpoint.
func (m *MemoryCheckpoint) Reset(unit string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.done, unit)
	return nil
}

// All implements Checkpoint.
func (m *MemoryCheckpoint) All() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.done))
	for k := range m.done {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// fileCheckpoint is a JSON sidecar Checkpoint. Mark calls do a full
// re-write; suitable for batch sizes up to ~10k units. For larger
// runs use a DB-backed checkpoint.
type fileCheckpoint struct {
	path string
	mu   sync.Mutex
	done map[string]struct{}
}

// FileCheckpoint loads (or creates) a JSON ledger at path.
func FileCheckpoint(path string) (Checkpoint, error) {
	fc := &fileCheckpoint{path: path, done: map[string]struct{}{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fc, nil
	}
	if err != nil {
		return nil, err
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	for _, u := range list {
		fc.done[u] = struct{}{}
	}
	return fc, nil
}

func (f *fileCheckpoint) Done(unit string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.done[unit]
	return ok, nil
}

func (f *fileCheckpoint) Mark(unit string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.done[unit] = struct{}{}
	return f.flushLocked()
}

func (f *fileCheckpoint) Reset(unit string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.done, unit)
	return f.flushLocked()
}

func (f *fileCheckpoint) All() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.done))
	for k := range f.done {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func (f *fileCheckpoint) flushLocked() error {
	all := make([]string, 0, len(f.done))
	for k := range f.done {
		all = append(all, k)
	}
	sort.Strings(all)
	data, err := json.Marshal(all)
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0o644)
}
