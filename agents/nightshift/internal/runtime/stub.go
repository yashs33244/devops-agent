package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

// StubLauncher is the in-process JobLauncher. It runs the configured
// worker binary as a subprocess per Launch and tracks the PID per
// runID so Interrupt can signal it.
//
// Intended uses:
//   - Unit / integration tests that want an end-to-end run lifecycle
//     without K8s.
//   - `NS_RUNTIME=stub` dev mode for local development.
type StubLauncher struct {
	// WorkerBinary is the absolute path of the worker executable.
	// Required.
	WorkerBinary string

	// ExtraArgs is passed to the worker after any launcher-managed
	// args. Typically empty.
	ExtraArgs []string

	mu      sync.Mutex
	running map[string]*exec.Cmd
	done    map[string]chan struct{}
	closed  bool
}

// NewStubLauncher constructs a stub launcher. Verifies binary exists.
func NewStubLauncher(workerBinary string) (*StubLauncher, error) {
	if workerBinary == "" {
		return nil, errors.New("runtime: stub launcher requires worker binary path")
	}
	info, err := os.Stat(workerBinary)
	if err != nil {
		return nil, fmt.Errorf("runtime: stat worker binary: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("runtime: worker binary path is a directory: %s", workerBinary)
	}
	return &StubLauncher{
		WorkerBinary: workerBinary,
		running:      map[string]*exec.Cmd{},
		done:         map[string]chan struct{}{},
	}, nil
}

// Launch starts the worker subprocess for spec.RunID.
func (s *StubLauncher) Launch(ctx context.Context, spec LaunchSpec) error {
	if spec.RunID == "" {
		return errors.New("runtime: LaunchSpec.RunID required")
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("runtime: stub launcher closed")
	}
	if _, ok := s.running[spec.RunID]; ok {
		s.mu.Unlock()
		return nil // idempotent
	}

	stateDir, err := prepareStubSessionState(spec)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("runtime: stub session-state: %w", err)
	}

	cmd := exec.Command(s.WorkerBinary, s.ExtraArgs...)
	cmd.Env = buildEnv(spec, stateDir)
	// Capture stdout/stderr for debugging; in production this would
	// flow to pod logs.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("runtime: stub launch: %w", err)
	}
	doneCh := make(chan struct{})
	s.running[spec.RunID] = cmd
	s.done[spec.RunID] = doneCh
	s.mu.Unlock()

	// Reap the subprocess in a background goroutine so subsequent
	// Interrupt calls find the cmd in the map until it exits.
	go func() {
		_ = cmd.Wait()
		s.mu.Lock()
		delete(s.running, spec.RunID)
		delete(s.done, spec.RunID)
		s.mu.Unlock()
		close(doneCh)
	}()
	return nil
}

// Interrupt sends SIGTERM to the worker subprocess. If the run isn't
// tracked, Interrupt is a no-op (per JobLauncher contract).
func (s *StubLauncher) Interrupt(ctx context.Context, runID string) error {
	s.mu.Lock()
	cmd, ok := s.running[runID]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// ESRCH = process already exited; acceptable.
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("runtime: stub interrupt signal: %w", err)
	}
	return nil
}

// Close stops all running subprocesses.
func (s *StubLauncher) Close() error {
	s.mu.Lock()
	s.closed = true
	cmds := make([]*exec.Cmd, 0, len(s.running))
	for _, c := range s.running {
		cmds = append(cmds, c)
	}
	s.mu.Unlock()
	for _, c := range cmds {
		if c.Process != nil {
			_ = c.Process.Kill()
		}
	}
	return nil
}

// Wait blocks until the specified runID has exited. For tests.
func (s *StubLauncher) Wait(runID string) {
	s.mu.Lock()
	ch, ok := s.done[runID]
	s.mu.Unlock()
	if !ok {
		return
	}
	<-ch
}

// buildEnv constructs the worker env per worker-protocol.md §2. If
// stateDir is non-empty, it is exported as NS_SESSION_STATE_DIR.
func buildEnv(spec LaunchSpec, stateDir string) []string {
	env := os.Environ()
	set := func(k, v string) { env = append(env, k+"="+v) }
	set("NS_RUN_ID", spec.RunID)
	set("NS_USER_ID", spec.UserID)
	set("NS_SESSION_ID", spec.SessionID)
	set("NS_PROMPT", spec.Prompt)
	set("NS_API_URL", spec.CallbackURL)
	set("NS_WORKER_CREDENTIAL", spec.WorkerCredential)
	if spec.ActiveDeadlineSeconds > 0 {
		set("NS_ACTIVE_DEADLINE_SECONDS", strconv.FormatInt(spec.ActiveDeadlineSeconds, 10))
	}
	if stateDir != "" {
		set("NS_SESSION_STATE_DIR", stateDir)
	}
	if spec.SDKSessionID != "" {
		set("NS_SDK_SESSION_ID", spec.SDKSessionID)
	}
	for k, v := range spec.ExtraEnv {
		set(k, v)
	}
	return env
}

// prepareStubSessionState resolves and provisions a per-session
// directory for the stub launcher when SessionState is configured
// with the host backend. Returns the absolute path to set as
// NS_SESSION_STATE_DIR, or "" when no session-state mount applies.
//
// Other backends (pvc, object) are no-ops in the stub launcher: pvc
// is a K8s concept; object round-trip is chunk 14.
func prepareStubSessionState(spec LaunchSpec) (string, error) {
	if spec.SessionState.Backend != SessionStateBackendHost {
		return "", nil
	}
	if spec.SessionState.HostRoot == "" {
		return "", nil
	}
	sub, err := SessionSubPath(spec.UserID, spec.SessionID)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(spec.SessionState.HostRoot, sub)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
