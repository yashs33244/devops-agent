# SRE Guard — AI Agent Context

## What this is

A Python `asyncio` daemon that runs as a persistent guard for deployed services. It:

1. **Monitors** Prometheus metrics, HTTP health endpoints, and (optionally) Kubernetes events on a configurable poll interval (default 30 s).
2. **Alerts** via stdout (rich-formatted), a JSONL log file in `/tmp/sre-guard-alerts/`, and Slack webhook.
3. **Diagnoses** incidents using `holmesgpt` (invoked as a subprocess, never imported as a library). Falls back to a rule-based heuristic analyser when holmesgpt is not installed.
4. **Accepts commands** via a FastAPI REST API on port 8888.
5. **Is controlled** via the `sreguard` CLI companion (`cli/sreguard.py`).

## Architecture

```
daemon.py          ← asyncio.run() entrypoint; wires MonitorLoop + uvicorn
  monitor.py       ← polling loop; AlertState per service; Prometheus + health checks
  commander.py     ← FastAPI app; injected at startup via bind()
  investigator.py  ← subprocess wrapper for holmesgpt + fallback analysis
  alerter.py       ← fire() coroutine; Slack + log-file + Rich stdout
  config.py        ← Pydantic SREGuardConfig; loads sre-guard.yaml
cli/sreguard.py    ← Click CLI; talks to daemon via httpx
```

## Key design decisions

- **No holmesgpt library import** — always called as `holmes ask "..." --output json` subprocess.
- **Graceful degradation** — if Kubernetes is unreachable, k8s checks are skipped. If Prometheus is unreachable, `None` is returned and the rule is skipped silently.
- **Per-service exception isolation** — `asyncio.gather(*tasks, return_exceptions=True)` in `tick()` means one broken service never crashes monitoring for others.
- **for_duration semantics** — an alert only fires after the condition has been continuously true for `for_duration` seconds (tracked via `AlertState._pending`).
- **Silence** is per-service and stored in memory (`MonitorLoop._silences`). Restarting the daemon clears all silences.
- **PID file** at `/tmp/sre-guard.pid` — used by the CLI for stop/restart.

## Running locally (development)

```bash
cd agents/sre-guard
pip install -e ".[dev]"

# Start daemon (foreground)
python -m sre_guard.daemon

# Or via CLI
python cli/sreguard.py daemon start --foreground

# Check status
python cli/sreguard.py status

# Run tests
pytest tests/ -v
```

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `SLACK_WEBHOOK_URL` | `""` | Slack incoming webhook for alert posting |
| `SRE_GUARD_PORT` | `8888` | Port CLI uses to connect to daemon |
| `SRE_GUARD_HOST` | `localhost` | Host CLI uses to connect to daemon |
| `SRE_GUARD_LOG_DIR` | `/tmp/sre-guard-alerts` | Directory for JSONL alert logs |

## REST API quick reference

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness probe |
| GET | `/status` | All services + active alerts |
| POST | `/watch` | Add a service |
| DELETE | `/watch/{service}` | Remove a service |
| POST | `/diagnose/{service}` | Trigger AI investigation |
| POST | `/silence/{service}` | Mute alerts for N minutes |
| POST | `/runbook/{service}` | Execute a runbook action (restart_pod, scale_up, …) |

## holmesgpt integration

`investigator.py` checks for the `holmes` binary at runtime via `shutil.which()`. If found, it calls:

```bash
holmes ask "investigate <service>: <context>" --output json
```

Output is parsed as JSON (keys: `findings`, `analysis`, or `result`). If the binary is absent or returns a non-zero exit code, a structured fallback analysis is returned instead.

## Adding a new alert rule

Edit `config/sre-guard.yaml` and add an entry under the relevant service's `alert_rules`:

```yaml
- name: MyRule
  query: 'my_metric{job="my-service"}'
  threshold: 10.0
  comparison: gt          # gt | lt | eq
  severity: warning       # critical | warning | info
  for_duration: 120       # seconds condition must be true before firing
```

Restart the daemon to pick up the new config.
