package scl

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestManagerAtomicReload(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "server.scl"), filepath.Join(dir, "theme.scl")
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(a, "name = \"old\"\n")
	write(b, "fps = 30\n")
	m, err := OpenManager([]FileSpec{
		{Name: "server", Path: a, Schema: Schema{"name": {Kind: String, Required: true, HotReload: true}}},
		{Name: "theme", Path: b, Schema: Schema{"fps": {Kind: Integer, Required: true, HotReload: true}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	write(a, "name = \"new\"\n")
	write(b, "fps = \"bad\"\n")
	if _, err := m.Reload(); err == nil {
		t.Fatal("invalid candidate published")
	}
	if v, _ := m.Get("server", "name"); v != "old" {
		t.Fatal("partial reload")
	}
	write(b, "fps = 60\n")
	check, err := m.CheckReload()
	if err != nil || len(check.Changed) != 2 || check.Generation != 1 {
		t.Fatalf("unexpected preflight: %+v %v", check, err)
	}
	if v, _ := m.Get("server", "name"); v != "old" {
		t.Fatal("preflight mutated snapshot")
	}
	report, err := m.Reload()
	if err != nil || report.Generation != 2 || len(report.Changed) != 2 {
		t.Fatalf("unexpected reload: %+v %v", report, err)
	}
	version, snapshot := m.Snapshot()
	if version != 2 || snapshot["server"]["name"] != "new" {
		t.Fatal("invalid snapshot")
	}
	snapshot["server"]["name"] = "external mutation"
	if v, _ := m.Get("server", "name"); v != "new" {
		t.Fatal("snapshot not isolated")
	}
	report, err = m.Reload()
	if err != nil || report.Generation != 2 || len(report.Changed) != 0 {
		t.Fatal("no-op reload changed generation")
	}
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Reload(); err == nil {
		t.Fatal("missing file accepted")
	}
	if v, ok := m.Get("theme", "fps"); !ok || v == nil {
		t.Fatal("last working values lost")
	}
}

func TestManagerRestartOnlyAndReaders(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.scl")
	if err := os.WriteFile(p, []byte("port = 2222\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := OpenManager([]FileSpec{{Name: "server", Path: p, Schema: Schema{"port": {Kind: Integer, Required: true}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("port = 2223\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				g, _ := m.Snapshot()
				if g != 1 {
					t.Errorf("unexpected generation %d", g)
				}
			}
		}()
	}
	_, err = m.Reload()
	wg.Wait()
	if err == nil || !strings.Contains(err.Error(), "requires a process restart") {
		t.Fatalf("restart-only change not rejected: %v", err)
	}
}
