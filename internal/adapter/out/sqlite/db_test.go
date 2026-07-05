package sqlite

import (
	"path/filepath"
	"testing"
)

// TestDefaultPath_MOMODataDirOverride verifies the escape hatch that lets
// two instances run in isolation on one machine (Phase 5's two-app LAN
// share manual verification, docs/plan/06-phase5-host-sharing.md §6).
func TestDefaultPath_MOMODataDirOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MOMO_DATA_DIR", dir)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("DefaultPath() = %q, want directory %q", path, dir)
	}
	if filepath.Base(path) != "data.db" {
		t.Fatalf("DefaultPath() base = %q, want data.db", filepath.Base(path))
	}
}

func TestDefaultPath_NoOverrideUsesUserConfigDir(t *testing.T) {
	t.Setenv("MOMO_DATA_DIR", "")

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if filepath.Base(filepath.Dir(path)) != "momo-shell" {
		t.Fatalf("DefaultPath() = %q, want parent dir named momo-shell", path)
	}
}
