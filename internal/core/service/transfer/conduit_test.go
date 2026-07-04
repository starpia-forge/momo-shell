package transfer

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestConduit_ReadReturnsPushedChunks(t *testing.T) {
	c := newConduit(func([]byte) error { return nil })
	c.push([]byte("hello"))
	c.push([]byte(" world"))

	buf := make([]byte, 64)
	var got []byte
	for len(got) < len("hello world") {
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		got = append(got, buf[:n]...)
	}
	if !bytes.Equal(got, []byte("hello world")) {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}

func TestConduit_ReadHonorsSmallBufferAcrossCalls(t *testing.T) {
	c := newConduit(func([]byte) error { return nil })
	c.push([]byte("abcdef"))

	buf := make([]byte, 2)
	var got []byte
	for len(got) < 6 {
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		got = append(got, buf[:n]...)
	}
	if !bytes.Equal(got, []byte("abcdef")) {
		t.Fatalf("got %q, want %q", got, "abcdef")
	}
}

func TestConduit_WriteCallsWriteFn(t *testing.T) {
	var written [][]byte
	c := newConduit(func(p []byte) error {
		cp := append([]byte{}, p...)
		written = append(written, cp)
		return nil
	})

	n, err := c.Write([]byte("frame"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len("frame") {
		t.Fatalf("n = %d, want %d", n, len("frame"))
	}
	if len(written) != 1 || !bytes.Equal(written[0], []byte("frame")) {
		t.Fatalf("writeFn saw %v", written)
	}
}

func TestConduit_CloseUnblocksRead(t *testing.T) {
	c := newConduit(func([]byte) error { return nil })
	done := make(chan error, 1)
	go func() {
		_, err := c.Read(make([]byte, 16))
		done <- err
	}()

	c.Close()
	select {
	case err := <-done:
		if err != io.EOF {
			t.Fatalf("expected io.EOF after Close, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not unblock after Close")
	}
}

func TestConduit_PushAfterCloseDoesNotBlock(t *testing.T) {
	c := newConduit(func([]byte) error { return nil })
	c.Close()

	done := make(chan struct{})
	go func() {
		c.push([]byte("late chunk"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("push blocked after Close")
	}
}
