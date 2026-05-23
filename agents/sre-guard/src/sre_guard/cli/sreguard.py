#!/usr/bin/env python3
"""sreguard CLI — control and query the SRE Guard daemon."""

from __future__ import annotations

import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Optional

import click
import httpx
from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.text import Text

console = Console()

_DEFAULT_PORT = int(os.getenv("SRE_GUARD_PORT", "8888"))
_DEFAULT_HOST = os.getenv("SRE_GUARD_HOST", "localhost")
_PID_FILE = Path("/tmp/sre-guard.pid")
_LOG_DIR = Path(os.getenv("SRE_GUARD_LOG_DIR", "/tmp/sre-guard-alerts"))


def _api_url(path: str, host: str = _DEFAULT_HOST, port: int = _DEFAULT_PORT) -> str:
    return f"http://{host}:{port}{path}"


def _get(path: str, **kwargs) -> dict:
    try:
        resp = httpx.get(_api_url(path), timeout=10.0, **kwargs)
        resp.raise_for_status()
        return resp.json()
    except httpx.ConnectError:
        console.print(
            "[bold red]Cannot connect to SRE Guard daemon.[/] "
            f"Is it running on {_DEFAULT_HOST}:{_DEFAULT_PORT}?"
        )
        sys.exit(1)
    except httpx.HTTPStatusError as exc:
        console.print(f"[bold red]API error {exc.response.status_code}:[/] {exc.response.text}")
        sys.exit(1)


def _post(path: str, json_body: dict | None = None) -> dict:
    try:
        resp = httpx.post(_api_url(path), json=json_body, timeout=30.0)
        resp.raise_for_status()
        return resp.json()
    except httpx.ConnectError:
        console.print(
            "[bold red]Cannot connect to SRE Guard daemon.[/] "
            f"Is it running on {_DEFAULT_HOST}:{_DEFAULT_PORT}?"
        )
        sys.exit(1)
    except httpx.HTTPStatusError as exc:
        console.print(f"[bold red]API error {exc.response.status_code}:[/] {exc.response.text}")
        sys.exit(1)


def _delete(path: str) -> dict:
    try:
        resp = httpx.delete(_api_url(path), timeout=10.0)
        resp.raise_for_status()
        return resp.json()
    except httpx.ConnectError:
        console.print("[bold red]Cannot connect to SRE Guard daemon.[/]")
        sys.exit(1)
    except httpx.HTTPStatusError as exc:
        console.print(f"[bold red]API error {exc.response.status_code}:[/] {exc.response.text}")
        sys.exit(1)


# ---------------------------------------------------------------------------
# CLI group
# ---------------------------------------------------------------------------

@click.group()
def cli():
    """SRE Guard — persistent service monitoring daemon."""


# ---------------------------------------------------------------------------
# status
# ---------------------------------------------------------------------------

@cli.command("status")
def cmd_status():
    """Show all watched services and their current alert state."""
    data = _get("/status")

    console.print(
        Panel(
            f"[bold green]Daemon:[/] {data.get('daemon', 'unknown')}  "
            f"| Poll: {data.get('poll_interval_seconds', '?')}s  "
            f"| Active alerts: {data.get('total_active_alerts', 0)}",
            title="SRE Guard Status",
        )
    )

    services = data.get("services", [])
    if not services:
        console.print("[dim]No services being watched.[/]")
        return

    table = Table(title="Watched Services", show_lines=True)
    table.add_column("Name", style="bold cyan")
    table.add_column("Namespace")
    table.add_column("Rules", justify="right")
    table.add_column("Active Alerts", justify="right")
    table.add_column("Silenced")

    for svc in services:
        alert_count = len(svc.get("active_alerts", []))
        alert_style = "bold red" if alert_count else "green"
        silenced_str = (
            f"[yellow]Yes (until {svc['silence_until']})[/]"
            if svc.get("silenced")
            else "[green]No[/]"
        )
        table.add_row(
            svc["name"],
            svc.get("namespace", "-"),
            str(svc.get("alert_rules", 0)),
            Text(str(alert_count), style=alert_style),
            silenced_str,
        )
    console.print(table)

    # Print active alert details
    for svc in services:
        for alert in svc.get("active_alerts", []):
            sev = alert.get("severity", "?").upper()
            color = {"CRITICAL": "red", "WARNING": "yellow", "INFO": "cyan"}.get(sev, "white")
            console.print(
                Panel(
                    f"[bold]Rule:[/] {alert.get('rule_name')}\n"
                    f"[bold]Message:[/] {alert.get('message')}\n"
                    f"[bold]Fired at:[/] {alert.get('fired_at')}",
                    title=f"[{color}][FIRING] {sev} — {svc['name']}[/]",
                    border_style=color,
                )
            )


# ---------------------------------------------------------------------------
# watch
# ---------------------------------------------------------------------------

@cli.command("watch")
@click.argument("service")
@click.option("--prometheus", default="http://prometheus:9090", show_default=True, help="Prometheus URL")
@click.option("--namespace", default="default", show_default=True)
@click.option("--health-url", default="", help="HTTP health endpoint URL")
def cmd_watch(service: str, prometheus: str, namespace: str, health_url: str):
    """Add SERVICE to the watch list."""
    result = _post(
        "/watch",
        {
            "service": service,
            "prometheus_url": prometheus,
            "namespace": namespace,
            "health_url": health_url,
        },
    )
    console.print(f"[green]Now watching:[/] {result.get('added', service)}")


# ---------------------------------------------------------------------------
# unwatch
# ---------------------------------------------------------------------------

@cli.command("unwatch")
@click.argument("service")
def cmd_unwatch(service: str):
    """Stop watching SERVICE."""
    result = _delete(f"/watch/{service}")
    console.print(f"[yellow]Stopped watching:[/] {result.get('removed', service)}")


# ---------------------------------------------------------------------------
# diagnose
# ---------------------------------------------------------------------------

@cli.command("diagnose")
@click.argument("service")
@click.option("--context", default="", help="Optional extra context for the investigation.")
def cmd_diagnose(service: str, context: str):
    """AI-powered investigation of SERVICE via holmesgpt."""
    console.print(f"[cyan]Investigating {service}…[/]")
    result = _post(f"/diagnose/{service}", {"context": context})
    findings = result.get("findings", "No findings returned.")
    try:
        parsed = json.loads(findings)
        console.print(
            Panel(json.dumps(parsed, indent=2), title=f"Diagnosis — {service}", border_style="cyan")
        )
    except (json.JSONDecodeError, TypeError):
        console.print(Panel(str(findings), title=f"Diagnosis — {service}", border_style="cyan"))


# ---------------------------------------------------------------------------
# silence
# ---------------------------------------------------------------------------

@cli.command("silence")
@click.argument("service")
@click.option("--minutes", default=30, show_default=True, help="Silence duration in minutes.")
def cmd_silence(service: str, minutes: int):
    """Mute alerts for SERVICE for N minutes."""
    result = _post(f"/silence/{service}", {"minutes": minutes})
    console.print(
        f"[yellow]Silenced {service} for {minutes}m — until {result.get('until')}[/]"
    )


# ---------------------------------------------------------------------------
# logs
# ---------------------------------------------------------------------------

@cli.command("logs")
@click.argument("service")
@click.option("--tail", default=50, show_default=True, help="Number of recent entries to show.")
def cmd_logs(service: str, tail: int):
    """Show recent alert log entries for SERVICE."""
    log_file = _LOG_DIR / f"{service}.jsonl"
    if not log_file.exists():
        console.print(f"[dim]No alert log found for {service} at {log_file}[/]")
        return

    lines: list[dict] = []
    with log_file.open() as fh:
        for line in fh:
            line = line.strip()
            if line:
                try:
                    lines.append(json.loads(line))
                except json.JSONDecodeError:
                    pass

    recent = lines[-tail:]
    if not recent:
        console.print(f"[dim]No entries found for {service}[/]")
        return

    table = Table(title=f"Alert Log — {service} (last {len(recent)})", show_lines=True)
    table.add_column("Fired At")
    table.add_column("Rule")
    table.add_column("Severity")
    table.add_column("Message", no_wrap=False)
    table.add_column("Resolved")

    for entry in recent:
        sev = entry.get("severity", "?").upper()
        color = {"CRITICAL": "red", "WARNING": "yellow", "INFO": "cyan"}.get(sev, "white")
        resolved = entry.get("resolved_at") or "[dim]ongoing[/]"
        table.add_row(
            entry.get("fired_at", "?"),
            entry.get("rule_name", "?"),
            Text(sev, style=color),
            entry.get("message", ""),
            resolved,
        )
    console.print(table)


# ---------------------------------------------------------------------------
# daemon subgroup
# ---------------------------------------------------------------------------

@cli.group("daemon")
def daemon_group():
    """Manage the SRE Guard daemon process."""


@daemon_group.command("start")
@click.option("--config", "config_path", default=None, help="Path to sre-guard.yaml")
@click.option("--background/--foreground", default=True, show_default=True)
def daemon_start(config_path: Optional[str], background: bool):
    """Start the SRE Guard daemon."""
    if _PID_FILE.exists():
        pid = _PID_FILE.read_text().strip()
        console.print(
            f"[yellow]Daemon appears already running (PID {pid}). "
            "Use 'sreguard daemon stop' first or remove /tmp/sre-guard.pid.[/]"
        )
        sys.exit(1)

    # Build the command
    cmd = [sys.executable, "-m", "sre_guard.daemon"]
    if config_path:
        cmd += ["--config", config_path]

    if background:
        proc = subprocess.Popen(
            cmd,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
        )
        console.print(f"[green]SRE Guard daemon started in background (PID {proc.pid})[/]")
    else:
        # Foreground — import and run directly
        from sre_guard.daemon import run
        run(config_path)


@daemon_group.command("stop")
def daemon_stop():
    """Stop the SRE Guard daemon."""
    if not _PID_FILE.exists():
        console.print("[dim]No PID file found — daemon may not be running.[/]")
        return
    pid_str = _PID_FILE.read_text().strip()
    try:
        pid = int(pid_str)
        os.kill(pid, signal.SIGTERM)
        # Wait up to 5s for clean shutdown
        for _ in range(10):
            time.sleep(0.5)
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                break
        console.print(f"[green]Sent SIGTERM to PID {pid}.[/]")
    except (ValueError, ProcessLookupError):
        console.print(f"[yellow]Process {pid_str} not found — cleaning up PID file.[/]")
        _PID_FILE.unlink(missing_ok=True)
    except PermissionError:
        console.print(f"[red]Permission denied sending SIGTERM to PID {pid_str}.[/]")


@daemon_group.command("restart")
@click.option("--config", "config_path", default=None, help="Path to sre-guard.yaml")
@click.pass_context
def daemon_restart(ctx: click.Context, config_path: Optional[str]):
    """Stop then start the SRE Guard daemon."""
    ctx.invoke(daemon_stop)
    time.sleep(1)
    ctx.invoke(daemon_start, config_path=config_path, background=True)


if __name__ == "__main__":
    cli()
