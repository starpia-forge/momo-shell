package localfs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return p
}

func TestListDir_ReturnsFileAndDirMetadata(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "b.txt", "hello")
	if err := os.Mkdir(filepath.Join(dir, "a-dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	svc := New()
	entries, err := svc.ListDir(dir)
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// dirs sort first, then alphabetical
	if entries[0].Name != "a-dir" || !entries[0].IsDir {
		t.Errorf("expected a-dir first, got %+v", entries[0])
	}
	if entries[1].Name != "b.txt" || entries[1].IsDir {
		t.Errorf("expected b.txt second, got %+v", entries[1])
	}
	if entries[1].Size != 5 {
		t.Errorf("expected size 5, got %d", entries[1].Size)
	}
}

func TestListDir_MissingDirErrors(t *testing.T) {
	svc := New()
	if _, err := svc.ListDir(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestStat_Found(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "f.txt", "content")

	svc := New()
	entry, ok, err := svc.Stat(p)
	if err != nil || !ok {
		t.Fatalf("Stat: ok=%v err=%v", ok, err)
	}
	if entry.Name != "f.txt" || entry.Size != int64(len("content")) {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestStat_NotFoundOkFalse(t *testing.T) {
	svc := New()
	_, ok, err := svc.Stat(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("expected nil error for not-found, got %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for missing path")
	}
}

func TestMkdir_Rename_Remove_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	svc := New()

	newDir := filepath.Join(dir, "child")
	if err := svc.Mkdir(newDir); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	renamed := filepath.Join(dir, "renamed")
	if err := svc.Rename(newDir, renamed); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := os.Stat(renamed); err != nil {
		t.Fatalf("expected renamed dir to exist: %v", err)
	}

	if err := svc.Remove(renamed); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(renamed); !os.IsNotExist(err) {
		t.Fatalf("expected renamed dir to be gone, err=%v", err)
	}
}

func TestRemove_NonEmptyDirErrors(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, sub, "x.txt", "x")

	svc := New()
	if err := svc.Remove(sub); err == nil {
		t.Fatal("expected error removing non-empty dir")
	}
}

func TestCopy_FileContentAndMultipleSources(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	f1 := writeFile(t, srcDir, "one.txt", "one-content")
	f2 := writeFile(t, srcDir, "two.txt", "two-content")

	svc := New()
	if err := svc.Copy([]string{f1, f2}, dstDir); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	got1, err := os.ReadFile(filepath.Join(dstDir, "one.txt"))
	if err != nil || string(got1) != "one-content" {
		t.Errorf("one.txt: got %q err %v", got1, err)
	}
	got2, err := os.ReadFile(filepath.Join(dstDir, "two.txt"))
	if err != nil || string(got2) != "two-content" {
		t.Errorf("two.txt: got %q err %v", got2, err)
	}
	// source preserved
	if _, err := os.Stat(f1); err != nil {
		t.Errorf("expected source to remain after copy: %v", err)
	}
}

func TestCopy_SamePathIsNoOpAndPreservesContent(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "same.txt", "keep-me")

	svc := New()
	if err := svc.Copy([]string{f}, dir); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read after self-copy: %v", err)
	}
	if string(got) != "keep-me" {
		t.Fatalf("self-copy corrupted the source: got %q", got)
	}
}

func TestCopy_DirectorySourceRejected(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	sub := filepath.Join(srcDir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	svc := New()
	if err := svc.Copy([]string{sub}, dstDir); err == nil {
		t.Fatal("expected error copying a directory")
	}
}

func TestMove_MovesFileAndRemovesSource(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	f := writeFile(t, srcDir, "m.txt", "move-me")

	svc := New()
	if err := svc.Move([]string{f}, dstDir); err != nil {
		t.Fatalf("Move: %v", err)
	}

	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Fatalf("expected source removed, err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(dstDir, "m.txt"))
	if err != nil || string(got) != "move-me" {
		t.Errorf("dst content: got %q err %v", got, err)
	}
}

func TestMove_IntoDirKeepsBasename(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	sub := filepath.Join(srcDir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	svc := New()
	if err := svc.Move([]string{sub}, dstDir); err != nil {
		t.Fatalf("Move dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "subdir")); err != nil {
		t.Fatalf("expected moved dir at dst: %v", err)
	}
}

func TestHomeDir_NonEmpty(t *testing.T) {
	svc := New()
	home, err := svc.HomeDir()
	if err != nil {
		t.Fatalf("HomeDir: %v", err)
	}
	if home == "" {
		t.Fatal("expected non-empty home dir")
	}
}

func TestRoots_NonEmptyAndAbsolute(t *testing.T) {
	svc := New()
	roots, err := svc.Roots()
	if err != nil {
		t.Fatalf("Roots: %v", err)
	}
	if len(roots) == 0 {
		t.Fatal("expected at least one root")
	}
	if runtime.GOOS != "windows" {
		if roots[0] != "/" {
			t.Errorf("expected unix root '/', got %q", roots[0])
		}
	}
}
