package transfer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"momo-shell/internal/core/port/in"
)

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func taskByID(svc *Service, id string) (in.TaskInfo, bool) {
	for _, info := range svc.Tasks() {
		if info.ID == id {
			return info, true
		}
	}
	return in.TaskInfo{}, false
}

func writeLocalFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	return p
}

func TestUpload_SingleFile_Success(t *testing.T) {
	remote := newFakeRemoteFS(t)
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	svc := New(Deps{Shell: shell})

	localDir := t.TempDir()
	localFile := writeLocalFile(t, localDir, "hello.txt", "hello world")

	ids, err := svc.Upload("s1", []string{localFile}, "", in.ConflictOverwrite)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 task, got %d", len(ids))
	}

	waitFor(t, time.Second, func() bool {
		info, ok := taskByID(svc, ids[0])
		return ok && info.State == in.TaskDone
	})

	got, err := os.ReadFile(remote.native("hello.txt"))
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if string(got) != "hello world" {
		t.Fatalf("uploaded content mismatch: got %q", got)
	}

	info, _ := taskByID(svc, ids[0])
	if info.Bytes != info.Total || info.Total != int64(len("hello world")) {
		t.Fatalf("unexpected byte accounting: %+v", info)
	}
}

func TestUpload_Directory_Recursive(t *testing.T) {
	remote := newFakeRemoteFS(t)
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	svc := New(Deps{Shell: shell})

	localDir := t.TempDir()
	root := filepath.Join(localDir, "project")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeLocalFile(t, root, "a.txt", "AAA")
	writeLocalFile(t, filepath.Join(root, "sub"), "b.txt", "BBBB")

	ids, err := svc.Upload("s1", []string{root}, "dest", in.ConflictOverwrite)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 task (one per top-level item), got %d", len(ids))
	}

	waitFor(t, time.Second, func() bool {
		info, ok := taskByID(svc, ids[0])
		return ok && info.State == in.TaskDone
	})

	if got, err := os.ReadFile(remote.native("dest/project/a.txt")); err != nil || string(got) != "AAA" {
		t.Fatalf("a.txt mismatch: content=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(remote.native("dest/project/sub/b.txt")); err != nil || string(got) != "BBBB" {
		t.Fatalf("sub/b.txt mismatch: content=%q err=%v", got, err)
	}

	info, _ := taskByID(svc, ids[0])
	if info.Total != int64(len("AAA")+len("BBBB")) {
		t.Fatalf("expected total to sum both files, got %d", info.Total)
	}
}

func TestUpload_ConflictPolicies(t *testing.T) {
	t.Run("overwrite replaces existing content", func(t *testing.T) {
		remote := newFakeRemoteFS(t)
		if err := os.WriteFile(remote.native("hello.txt"), []byte("old"), 0o644); err != nil {
			t.Fatalf("seed remote file: %v", err)
		}
		shell := newFakeShellAccess()
		shell.set("s1", remote)
		svc := New(Deps{Shell: shell})

		localFile := writeLocalFile(t, t.TempDir(), "hello.txt", "new")
		ids, err := svc.Upload("s1", []string{localFile}, "", in.ConflictOverwrite)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}
		waitFor(t, time.Second, func() bool {
			info, ok := taskByID(svc, ids[0])
			return ok && info.State == in.TaskDone
		})
		got, _ := os.ReadFile(remote.native("hello.txt"))
		if string(got) != "new" {
			t.Fatalf("expected overwrite, got %q", got)
		}
	})

	t.Run("skip omits the task entirely", func(t *testing.T) {
		remote := newFakeRemoteFS(t)
		if err := os.WriteFile(remote.native("hello.txt"), []byte("old"), 0o644); err != nil {
			t.Fatalf("seed remote file: %v", err)
		}
		shell := newFakeShellAccess()
		shell.set("s1", remote)
		svc := New(Deps{Shell: shell})

		localFile := writeLocalFile(t, t.TempDir(), "hello.txt", "new")
		ids, err := svc.Upload("s1", []string{localFile}, "", in.ConflictSkip)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}
		if len(ids) != 0 {
			t.Fatalf("expected skip to produce no task, got %d", len(ids))
		}
		got, _ := os.ReadFile(remote.native("hello.txt"))
		if string(got) != "old" {
			t.Fatalf("expected file untouched, got %q", got)
		}
	})

	t.Run("rename appends a numeric suffix", func(t *testing.T) {
		remote := newFakeRemoteFS(t)
		if err := os.WriteFile(remote.native("hello.txt"), []byte("old"), 0o644); err != nil {
			t.Fatalf("seed remote file: %v", err)
		}
		shell := newFakeShellAccess()
		shell.set("s1", remote)
		svc := New(Deps{Shell: shell})

		localFile := writeLocalFile(t, t.TempDir(), "hello.txt", "new")
		ids, err := svc.Upload("s1", []string{localFile}, "", in.ConflictRename)
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}
		waitFor(t, time.Second, func() bool {
			info, ok := taskByID(svc, ids[0])
			return ok && info.State == in.TaskDone
		})
		if got, err := os.ReadFile(remote.native("hello (1).txt")); err != nil || string(got) != "new" {
			t.Fatalf("expected renamed file, content=%q err=%v", got, err)
		}
		if got, _ := os.ReadFile(remote.native("hello.txt")); string(got) != "old" {
			t.Fatalf("expected original untouched, got %q", got)
		}
	})
}

func TestDownload_SingleFile_Success(t *testing.T) {
	remote := newFakeRemoteFS(t)
	if err := os.WriteFile(remote.native("report.txt"), []byte("remote data"), 0o644); err != nil {
		t.Fatalf("seed remote file: %v", err)
	}
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	svc := New(Deps{Shell: shell})

	localDir := t.TempDir()
	ids, err := svc.Download("s1", []string{"report.txt"}, localDir, in.ConflictOverwrite)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	waitFor(t, time.Second, func() bool {
		info, ok := taskByID(svc, ids[0])
		return ok && info.State == in.TaskDone
	})

	got, err := os.ReadFile(filepath.Join(localDir, "report.txt"))
	if err != nil || string(got) != "remote data" {
		t.Fatalf("downloaded content mismatch: %q err=%v", got, err)
	}
}

func TestQueue_MaxTwoConcurrentPerSession(t *testing.T) {
	remote := newFakeRemoteFS(t)
	gate := make(chan struct{})
	gated := &gatedFS{RemoteFileSystem: remote, gate: gate}
	shell := newFakeShellAccess()
	shell.set("s1", gated)
	svc := New(Deps{Shell: shell})

	localDir := t.TempDir()
	files := []string{
		writeLocalFile(t, localDir, "f0.txt", "aaa"),
		writeLocalFile(t, localDir, "f1.txt", "bbb"),
		writeLocalFile(t, localDir, "f2.txt", "ccc"),
	}

	ids, err := svc.Upload("s1", files, "", in.ConflictOverwrite)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(ids))
	}

	waitFor(t, time.Second, func() bool { return gated.startedCount() == 2 })
	time.Sleep(30 * time.Millisecond)
	if got := gated.startedCount(); got != 2 {
		t.Fatalf("expected exactly 2 concurrently started, got %d", got)
	}

	close(gate)

	waitFor(t, time.Second, func() bool {
		for _, id := range ids {
			info, ok := taskByID(svc, id)
			if !ok || info.State != in.TaskDone {
				return false
			}
		}
		return true
	})
}

func TestCancel_QueuedTask_ImmediatelyCanceled(t *testing.T) {
	remote := newFakeRemoteFS(t)
	gate := make(chan struct{})
	gated := &gatedFS{RemoteFileSystem: remote, gate: gate}
	shell := newFakeShellAccess()
	shell.set("s1", gated)
	svc := New(Deps{Shell: shell})

	localDir := t.TempDir()
	files := []string{
		writeLocalFile(t, localDir, "f0.txt", "aaa"),
		writeLocalFile(t, localDir, "f1.txt", "bbb"),
		writeLocalFile(t, localDir, "f2.txt", "ccc"),
	}

	ids, err := svc.Upload("s1", files, "", in.ConflictOverwrite)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	waitFor(t, time.Second, func() bool { return gated.startedCount() == 2 })

	if err := svc.Cancel(ids[2]); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	info, ok := taskByID(svc, ids[2])
	if !ok || info.State != in.TaskCanceled {
		t.Fatalf("expected third task canceled immediately, got %+v", info)
	}

	close(gate)
	waitFor(t, time.Second, func() bool {
		i0, _ := taskByID(svc, ids[0])
		i1, _ := taskByID(svc, ids[1])
		return i0.State == in.TaskDone && i1.State == in.TaskDone
	})

	if got := gated.startedCount(); got != 2 {
		t.Fatalf("canceled queued task must never start, started=%d", got)
	}
}

func TestCancel_RunningTask_StopsMidCopy(t *testing.T) {
	remote := newFakeRemoteFS(t)
	gate := make(chan struct{})
	gated := &gatedFS{RemoteFileSystem: remote, gate: gate}
	shell := newFakeShellAccess()
	shell.set("s1", gated)
	svc := New(Deps{Shell: shell})

	// Large enough to span multiple copyBufSize chunks so cancellation is
	// observed before the whole file is copied.
	big := make([]byte, copyBufSize*4)
	localDir := t.TempDir()
	localFile := filepath.Join(localDir, "big.bin")
	if err := os.WriteFile(localFile, big, 0o644); err != nil {
		t.Fatalf("write big file: %v", err)
	}

	ids, err := svc.Upload("s1", []string{localFile}, "", in.ConflictOverwrite)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	waitFor(t, time.Second, func() bool { return gated.startedCount() == 1 })

	if err := svc.Cancel(ids[0]); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	close(gate) // let the first blocked write proceed so the loop can observe cancellation

	waitFor(t, time.Second, func() bool {
		info, ok := taskByID(svc, ids[0])
		return ok && info.State == in.TaskCanceled
	})

	info, _ := taskByID(svc, ids[0])
	if info.Bytes >= info.Total {
		t.Fatalf("expected partial copy on cancel, got bytes=%d total=%d", info.Bytes, info.Total)
	}
}

func TestFileSystemUnavailable_ReturnsError(t *testing.T) {
	shell := newFakeShellAccess()
	svc := New(Deps{Shell: shell})

	if _, err := svc.Upload("missing", []string{"/nonexistent"}, "", in.ConflictOverwrite); err == nil {
		t.Fatalf("expected error for session without a file system")
	}
}

func TestPublishesTaskAndProgressEvents(t *testing.T) {
	remote := newFakeRemoteFS(t)
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	pub := &recordingPublisher{}
	svc := New(Deps{Shell: shell, Pub: pub})

	localFile := writeLocalFile(t, t.TempDir(), "hello.txt", "hello world")
	ids, err := svc.Upload("s1", []string{localFile}, "", in.ConflictOverwrite)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		info, ok := taskByID(svc, ids[0])
		return ok && info.State == in.TaskDone
	})

	var sawTask, sawProgress bool
	for _, e := range pub.all() {
		if e.topic == "transfer:task" {
			sawTask = true
		}
		if e.topic == "transfer:progress:"+ids[0] {
			sawProgress = true
		}
	}
	if !sawTask {
		t.Fatalf("expected at least one transfer:task event")
	}
	if !sawProgress {
		t.Fatalf("expected at least one transfer:progress event")
	}
}

func TestCopyRemote_CopiesFileContent(t *testing.T) {
	remote := newFakeRemoteFS(t)
	if err := os.WriteFile(remote.native("src.txt"), []byte("copy me"), 0o644); err != nil {
		t.Fatalf("seed remote file: %v", err)
	}
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	svc := New(Deps{Shell: shell})

	if err := svc.CopyRemote("s1", []string{"src.txt"}, "dst"); err != nil {
		t.Fatalf("CopyRemote: %v", err)
	}

	got, err := os.ReadFile(remote.native("dst/src.txt"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(got) != "copy me" {
		t.Fatalf("copied content mismatch: got %q", got)
	}
	if _, err := os.Stat(remote.native("src.txt")); err != nil {
		t.Fatalf("expected source to remain after copy: %v", err)
	}
}

func TestCopyRemote_MultipleSources(t *testing.T) {
	remote := newFakeRemoteFS(t)
	if err := os.WriteFile(remote.native("a.txt"), []byte("aaa"), 0o644); err != nil {
		t.Fatalf("seed a.txt: %v", err)
	}
	if err := os.WriteFile(remote.native("b.txt"), []byte("bbb"), 0o644); err != nil {
		t.Fatalf("seed b.txt: %v", err)
	}
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	svc := New(Deps{Shell: shell})

	if err := svc.CopyRemote("s1", []string{"a.txt", "b.txt"}, "dst"); err != nil {
		t.Fatalf("CopyRemote: %v", err)
	}

	gotA, errA := os.ReadFile(remote.native("dst/a.txt"))
	gotB, errB := os.ReadFile(remote.native("dst/b.txt"))
	if errA != nil || string(gotA) != "aaa" {
		t.Errorf("a.txt: got %q err %v", gotA, errA)
	}
	if errB != nil || string(gotB) != "bbb" {
		t.Errorf("b.txt: got %q err %v", gotB, errB)
	}
}

func TestCopyRemote_SamePathIsNoOpAndPreservesContent(t *testing.T) {
	remote := newFakeRemoteFS(t)
	if err := os.WriteFile(remote.native("same.txt"), []byte("keep-me"), 0o644); err != nil {
		t.Fatalf("seed remote file: %v", err)
	}
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	svc := New(Deps{Shell: shell})

	if err := svc.CopyRemote("s1", []string{"same.txt"}, ""); err != nil {
		t.Fatalf("CopyRemote: %v", err)
	}

	got, err := os.ReadFile(remote.native("same.txt"))
	if err != nil {
		t.Fatalf("read after self-copy: %v", err)
	}
	if string(got) != "keep-me" {
		t.Fatalf("self-copy corrupted the source: got %q", got)
	}
}

func TestCopyRemote_DirectorySourceErrors(t *testing.T) {
	remote := newFakeRemoteFS(t)
	if err := os.Mkdir(remote.native("adir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	svc := New(Deps{Shell: shell})

	if err := svc.CopyRemote("s1", []string{"adir"}, "dst"); err == nil {
		t.Fatal("expected error copying a directory")
	}
}

func TestCopyRemote_MissingSourceErrors(t *testing.T) {
	remote := newFakeRemoteFS(t)
	shell := newFakeShellAccess()
	shell.set("s1", remote)
	svc := New(Deps{Shell: shell})

	if err := svc.CopyRemote("s1", []string{"nope.txt"}, "dst"); err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestCopyRemote_UnknownSessionErrors(t *testing.T) {
	shell := newFakeShellAccess()
	svc := New(Deps{Shell: shell})

	if err := svc.CopyRemote("missing", []string{"a.txt"}, "dst"); err == nil {
		t.Fatal("expected error for unknown session")
	}
}
