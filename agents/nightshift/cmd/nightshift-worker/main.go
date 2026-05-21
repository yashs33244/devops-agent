// Command nightshift-worker is the reference worker image. It speaks
// the agent-to-platform protocol defined in
// protos/nightshift/v1/worker-protocol.md.
//
// This implementation is intentionally simulation-only: it does not
// call any LLM. Its purpose is to prove the protocol end-to-end — a
// conforming run goes from CreateRun through StreamRunEvents to a
// terminal COMPLETED state without any SDK in the loop.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	nsv1 "github.com/nightshiftco/nightshift/gen/go/nightshift/v1"
)

type config struct {
	RunID            string
	UserID           string
	SessionID        string
	Prompt           string
	APIURL           string
	WorkerCredential string
	CancelPollSecs   int
}

func loadConfig() (*config, error) {
	c := &config{
		RunID:            os.Getenv("NS_RUN_ID"),
		UserID:           os.Getenv("NS_USER_ID"),
		SessionID:        os.Getenv("NS_SESSION_ID"),
		Prompt:           os.Getenv("NS_PROMPT"),
		APIURL:           os.Getenv("NS_API_URL"),
		WorkerCredential: os.Getenv("NS_WORKER_CREDENTIAL"),
		CancelPollSecs:   5,
	}
	if s := os.Getenv("NS_CANCEL_POLL_SECONDS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("NS_CANCEL_POLL_SECONDS: %w", err)
		}
		c.CancelPollSecs = n
	}
	for name, v := range map[string]string{
		"NS_RUN_ID":            c.RunID,
		"NS_API_URL":           c.APIURL,
		"NS_WORKER_CREDENTIAL": c.WorkerCredential,
	} {
		if v == "" {
			return nil, fmt.Errorf("%s is required", name)
		}
	}
	return c, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logger = logger.With("run_id", cfg.RunID, "user_id", cfg.UserID)
	logger.Info("nightshift-worker starting",
		"api", cfg.APIURL,
		"session_id", cfg.SessionID,
		"cancel_poll", cfg.CancelPollSecs)

	// When NS_SESSION_STATE_DIR is set, drop a sentinel proving the
	// volume is mounted + writable. The reference worker is
	// simulation-only and doesn't carry real session state; chunk 14
	// (nightshift-worker-claude) round-trips actual SDK state here.
	if dir := os.Getenv("NS_SESSION_STATE_DIR"); dir != "" {
		if err := writeSessionSentinel(dir, cfg); err != nil {
			logger.Warn("session-state sentinel write failed", "dir", dir, "err", err)
		} else {
			logger.Info("session-state sentinel written", "dir", dir)
		}
	}

	c, err := dial(cfg.APIURL, cfg.RunID, cfg.WorkerCredential)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1) system.worker_started
	if err := c.emit(ctx, "system.worker_started", map[string]any{
		"image":   "nightshift-worker",
		"version": "0.1.0-ref",
	}); err != nil {
		return failAndExit(ctx, c, logger, err)
	}

	// Short beat so a subscriber that joined at CreateRun has time to
	// see the first event via replay vs. live.
	if cancelled, err := checkCancel(ctx, c); err != nil || cancelled {
		return terminal(ctx, c, logger, cancelled, err)
	}

	// 2) assistant turn (fake content)
	assistantText := "hello from nightshift-worker — simulated reply to: " + truncate(cfg.Prompt, 200)
	if err := c.emit(ctx, "assistant", map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": assistantText},
		},
		"model": "simulation-0",
	}); err != nil {
		return failAndExit(ctx, c, logger, err)
	}

	if cancelled, err := checkCancel(ctx, c); err != nil || cancelled {
		return terminal(ctx, c, logger, cancelled, err)
	}

	// 3) result.success (synthetic usage)
	sdkSessionID := cfg.SessionID
	if sdkSessionID == "" {
		sdkSessionID = "sim-" + cfg.RunID
	}
	usage := &nsv1.RunUsage{
		InputTokens:         42,
		OutputTokens:        17,
		CacheReadTokens:     0,
		CacheCreationTokens: 0,
		TotalCostUsd:        0.000,
	}
	if err := c.emit(ctx, "result.success", map[string]any{
		"subtype":        "success",
		"session_id":     sdkSessionID,
		"usage":          map[string]any{"input_tokens": 42.0, "output_tokens": 17.0, "cache_read_tokens": 0.0, "cache_creation_tokens": 0.0},
		"total_cost_usd": 0.0,
		"duration_ms":    100.0,
	}); err != nil {
		return failAndExit(ctx, c, logger, err)
	}

	// 4) terminal CompleteRun
	if err := c.complete(ctx, sdkSessionID, usage); err != nil {
		return fmt.Errorf("CompleteRun: %w", err)
	}
	logger.Info("nightshift-worker completed cleanly")
	return nil
}

func checkCancel(ctx context.Context, c *client) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.pollCancellation(ctx)
}

// terminal handles the cancellation branch: on cancellation, emit a
// best-effort CompleteRun with empty session_id so the server
// observes a clean terminal transition (INTERRUPTED flows through
// the cancelled attribute + CompleteRun, not through FailRun).
func terminal(ctx context.Context, c *client, logger *slog.Logger, cancelled bool, err error) error {
	if err != nil {
		return failAndExit(ctx, c, logger, err)
	}
	logger.Info("cancellation observed; completing with empty usage")
	if cerr := c.complete(ctx, "", &nsv1.RunUsage{}); cerr != nil {
		return fmt.Errorf("CompleteRun after cancel: %w", cerr)
	}
	return nil
}

func failAndExit(ctx context.Context, c *client, logger *slog.Logger, cause error) error {
	logger.Error("worker encountered error; reporting to API", "cause", cause)
	if err := c.fail(ctx, cause.Error()); err != nil {
		logger.Error("FailRun itself failed", "err", err)
	}
	return cause
}

func writeSessionSentinel(dir string, cfg *config) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	payload := map[string]any{
		"run_id":     cfg.RunID,
		"user_id":    cfg.UserID,
		"session_id": cfg.SessionID,
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "started.json"), data, 0o600)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
