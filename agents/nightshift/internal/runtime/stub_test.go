package runtime

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// The stub launcher tests use a small shell-based worker stand-in so
// they don't depend on the cmd/nightshift-worker binary (which lives
// in a later task). The real worker gets exercised by the chunk-8b
// integration test.

func writeShellWorker(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub launcher tests require a POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-worker.sh")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func TestStubLaunchExitsCleanly(t *testing.T) {
	bin := writeShellWorker(t, `
echo "RUN_ID=$NS_RUN_ID" >&2
echo "PROMPT=$NS_PROMPT" >&2
exit 0
`)
	s, err := NewStubLauncher(bin)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer s.Close()

	if err := s.Launch(context.Background(), LaunchSpec{
		RunID:  "run-1",
		UserID: "alice",
		Prompt: "hi",
	}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	s.Wait("run-1")
}

func TestStubLaunchIdempotent(t *testing.T) {
	bin := writeShellWorker(t, "sleep 1; exit 0")
	s, err := NewStubLauncher(bin)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer s.Close()

	if err := s.Launch(context.Background(), LaunchSpec{RunID: "r"}); err != nil {
		t.Fatalf("launch 1: %v", err)
	}
	if err := s.Launch(context.Background(), LaunchSpec{RunID: "r"}); err != nil {
		t.Fatalf("launch 2 (idempotent): %v", err)
	}
	s.Wait("r")
}

func TestStubInterrupt(t *testing.T) {
	bin := writeShellWorker(t, `
# block on SIGTERM
trap 'exit 143' TERM
while true; do sleep 1; done
`)
	s, err := NewStubLauncher(bin)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer s.Close()

	if err := s.Launch(context.Background(), LaunchSpec{RunID: "r"}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	// Give the subprocess a moment to install the trap.
	time.Sleep(200 * time.Millisecond)
	if err := s.Interrupt(context.Background(), "r"); err != nil {
		t.Fatalf("interrupt: %v", err)
	}

	// Should exit within a short window.
	done := make(chan struct{})
	go func() {
		s.Wait("r")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("worker did not exit after Interrupt")
	}
}

func TestStubInterruptUnknownRunID(t *testing.T) {
	bin := writeShellWorker(t, "exit 0")
	s, err := NewStubLauncher(bin)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer s.Close()
	if err := s.Interrupt(context.Background(), "nope"); err != nil {
		t.Fatalf("interrupt missing: %v", err)
	}
}

func TestStubBadBinary(t *testing.T) {
	if _, err := NewStubLauncher(""); err == nil {
		t.Fatalf("empty path: expected error")
	}
	if _, err := NewStubLauncher(t.TempDir()); err == nil {
		t.Fatalf("dir path: expected error")
	}
	if _, err := NewStubLauncher("/nonexistent/worker-abcdef"); err == nil {
		t.Fatalf("nonexistent: expected error")
	}
}
