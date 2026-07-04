package zmodem

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"momo-shell/internal/core/port/out"
)

// pipeConn connects a Send and a Receive engine bidirectionally over real OS
// pipes (kernel-buffered, unlike io.Pipe's synchronous rendezvous) -- both
// sides open by writing their own header before reading the other's, which
// an unbuffered pipe would deadlock on.
type pipeConn struct {
	r *os.File
	w *os.File
}

func (p *pipeConn) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipeConn) Write(b []byte) (int, error) { return p.w.Write(b) }

func newPipePair() (a, b *pipeConn) {
	r1, w1, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	r2, w2, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	return &pipeConn{r: r1, w: w2}, &pipeConn{r: r2, w: w1}
}

func TestLoopback_SingleFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	content := bytes.Repeat([]byte("zmodem loopback test data. "), 200) // > one subpacket
	srcPath := filepath.Join(srcDir, "hello.txt")
	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	senderSide, receiverSide := newPipePair()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	engine := New()
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Send(ctx, senderSide, []string{srcPath}, nil)
	}()

	saved, err := engine.Receive(ctx, receiverSide, dstDir, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(saved) != 1 {
		t.Fatalf("expected 1 saved file, got %d: %v", len(saved), saved)
	}
	got, err := os.ReadFile(saved[0])
	if err != nil {
		t.Fatalf("read received file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
}

func TestLoopback_EmptyFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "empty.txt")
	if err := os.WriteFile(srcPath, nil, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	senderSide, receiverSide := newPipePair()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	engine := New()
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Send(ctx, senderSide, []string{srcPath}, nil)
	}()

	saved, err := engine.Receive(ctx, receiverSide, dstDir, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved file, got %d", len(saved))
	}
	info, err := os.Stat(saved[0])
	if err != nil {
		t.Fatalf("stat received file: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty file, got %d bytes", info.Size())
	}
}

func TestLoopback_MultipleFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	files := []string{"a.txt", "b.txt", "c.txt"}
	var paths []string
	contents := map[string][]byte{}
	for i, name := range files {
		p := filepath.Join(srcDir, name)
		content := bytes.Repeat([]byte{byte('a' + i)}, 100+i*50)
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		paths = append(paths, p)
		contents[name] = content
	}

	senderSide, receiverSide := newPipePair()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	engine := New()
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Send(ctx, senderSide, paths, nil)
	}()

	saved, err := engine.Receive(ctx, receiverSide, dstDir, nil)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(saved) != len(files) {
		t.Fatalf("expected %d saved files, got %d", len(files), len(saved))
	}
	for _, p := range saved {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		want := contents[filepath.Base(p)]
		if !bytes.Equal(got, want) {
			t.Fatalf("content mismatch for %s", p)
		}
	}
}

func TestLoopback_ProgressReported(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	content := bytes.Repeat([]byte("x"), copyBufSizeForTest())
	srcPath := filepath.Join(srcDir, "big.bin")
	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	senderSide, receiverSide := newPipePair()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	engine := New()
	var lastSenderProgress out.TransferProgress
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Send(ctx, senderSide, []string{srcPath}, func(p out.TransferProgress) {
			lastSenderProgress = p
		})
	}()

	var lastReceiverProgress out.TransferProgress
	_, err := engine.Receive(ctx, receiverSide, dstDir, func(p out.TransferProgress) {
		lastReceiverProgress = p
	})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Send: %v", err)
	}

	if lastSenderProgress.Bytes != int64(len(content)) {
		t.Fatalf("sender progress: got %d bytes, want %d", lastSenderProgress.Bytes, len(content))
	}
	if lastReceiverProgress.Bytes != int64(len(content)) {
		t.Fatalf("receiver progress: got %d bytes, want %d", lastReceiverProgress.Bytes, len(content))
	}
}

func copyBufSizeForTest() int { return subpacketMaxLen*3 + 17 }
