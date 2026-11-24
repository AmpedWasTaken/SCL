package scl

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
)

// FileSpec explicitly registers a local configuration file and its schema.
// Names are stable application-level identifiers, not paths supplied by users.
type FileSpec struct {
	Name   string
	Path   string
	Schema Schema
}

// ReloadReport describes a successful check or reload without exposing values.
type ReloadReport struct {
	Generation uint64
	Files      int
	Changed    []string
	Changes    []Change
}

// Manager owns a single effective snapshot spanning all registered files.
// A reload publishes all candidates together or leaves the previous snapshot
// untouched. Applications should read a single Snapshot per operation.
type Manager struct {
	reloadMu sync.Mutex
	mu       sync.RWMutex
	specs    []FileSpec
	values   map[string]object
	version  uint64
	subscribers map[chan uint64]struct{}
}

const MaxManagedFiles = 32

// OpenManager loads and validates every registered file before returning.
func OpenManager(specs []FileSpec) (*Manager, error) {
	if len(specs) == 0 || len(specs) > MaxManagedFiles {
		return nil, fmt.Errorf("register between 1 and %d configuration files", MaxManagedFiles)
	}
	m := &Manager{}
	names, paths := map[string]bool{}, map[string]bool{}
	for _, spec := range specs {
		if !validSegment(spec.Name) || spec.Path == "" || names[spec.Name] {
			return nil, fmt.Errorf("invalid or duplicate configuration registration %q", spec.Name)
		}
		path, err := filepath.Abs(spec.Path)
		if err != nil {
			return nil, err
		}
		if paths[path] {
			return nil, fmt.Errorf("duplicate configuration file %q", spec.Name)
		}
		schema, err := copySchema(spec.Schema)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.Name, err)
		}
		names[spec.Name], paths[path] = true, true
		m.specs = append(m.specs, FileSpec{Name: spec.Name, Path: path, Schema: schema})
	}
	sort.Slice(m.specs, func(i, j int) bool { return m.specs[i].Name < m.specs[j].Name })
	values, err := m.readAll()
	if err != nil {
		return nil, err
	}
	m.values, m.version = values, 1
	return m, nil
}

func (m *Manager) readAll() (map[string]object, error) {
	next := make(map[string]object, len(m.specs))
	for _, spec := range m.specs {
		d, err := Load(spec.Path, spec.Schema)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.Name, err)
		}
		next[spec.Name] = d.values
	}
	return next, nil
}

// Snapshot returns a consistent, detached copy of all effective values and
// its generation. Mutating the returned maps does not affect the manager.
func (m *Manager) Snapshot() (uint64, map[string]map[string]any) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]map[string]any, len(m.values))
	for name, v := range m.values {
		out[name] = clone(v).(object)
	}
	return m.version, out
}

func (m *Manager) Get(name, path string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := lookup(m.values[name], path)
	return clone(v), ok
}

func (m *Manager) prepare(publish bool) (ReloadReport, error) {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	next, err := m.readAll()
	if err != nil {
		return ReloadReport{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	report := ReloadReport{Generation: m.version, Files: len(m.specs)}
	for _, spec := range m.specs {
		old, fresh := m.values[spec.Name], next[spec.Name]
		if reflect.DeepEqual(old, fresh) {
			continue
		}
		for _, path := range spec.Schema.Paths() {
			field := spec.Schema[path]
			if field.HotReload {
				continue
			}
			a, aOK := lookup(old, path)
			b, bOK := lookup(fresh, path)
			if aOK != bOK || !reflect.DeepEqual(a, b) {
				return ReloadReport{}, fmt.Errorf("%s.%s requires a process restart; active configuration unchanged", spec.Name, path)
			}
		}
		report.Changed = append(report.Changed, spec.Name)
		report.Changes = append(report.Changes, diffObjects(spec.Name, nil, old, fresh)... )
	}
	if publish && len(report.Changed) != 0 {
		m.values = next
		m.version++
		report.Generation = m.version
		m.notifyLocked()
	}
	return report, nil
}

// CheckReload validates the candidate snapshot without publishing it.
func (m *Manager) CheckReload() (ReloadReport, error) {
	return m.prepare(false)
}

// Reload re-reads every registered file. Invalid files or modifications to
// restart-only fields reject the entire reload. No files are written.
func (m *Manager) Reload() (ReloadReport, error) {
	return m.prepare(true)
}
