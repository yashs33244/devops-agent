"""Alerter — fires alerts to Slack webhook, log file, and stdout."""

from __future__ import annotations

import asyncio
import json
import logging
import os
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

import httpx
from rich.console import Console
from rich.panel import Panel
from rich.text import Text

logger = logging.getLogger(__name__)
console = Console(stderr=True)

_SEVERITY_COLORS = {
    "critical": "bold red",
    "warning": "bold yellow",
    "info": "bold cyan",
}

_LOG_DIR = Path(os.getenv("SRE_GUARD_LOG_DIR", "/tmp/sre-guard-alerts"))


class Alert:
    def __init__(
        self,
        service: str,
        rule_name: str,
        severity: str,
        message: str,
        fired_at: Optional[datetime] = None,
        resolved_at: Optional[datetime] = None,
    ) -> None:
        self.service = service
        self.rule_name = rule_name
        self.severity = severity
        self.message = message
        self.fired_at: datetime = fired_at or datetime.now(tz=timezone.utc)
        self.resolved_at: Optional[datetime] = resolved_at

    def to_dict(self) -> dict:
        return {
            "service": self.service,
            "rule_name": self.rule_name,
            "severity": self.severity,
            "message": self.message,
            "fired_at": self.fired_at.isoformat(),
            "resolved_at": self.resolved_at.isoformat() if self.resolved_at else None,
        }

    def __str__(self) -> str:
        state = "RESOLVED" if self.resolved_at else "FIRING"
        return (
            f"[{state}] {self.severity.upper()} | {self.service} | "
            f"{self.rule_name} | {self.message}"
        )


def _ensure_log_dir() -> Path:
    _LOG_DIR.mkdir(parents=True, exist_ok=True)
    return _LOG_DIR


def _log_to_file(alert: Alert) -> None:
    log_dir = _ensure_log_dir()
    log_file = log_dir / f"{alert.service}.jsonl"
    try:
        with log_file.open("a") as fh:
            fh.write(json.dumps(alert.to_dict()) + "\n")
    except OSError as exc:
        logger.warning("Failed to write alert to %s: %s", log_file, exc)


def _print_rich(alert: Alert) -> None:
    color = _SEVERITY_COLORS.get(alert.severity.lower(), "white")
    state = "RESOLVED" if alert.resolved_at else "FIRING"
    title = Text(f"[{state}] {alert.severity.upper()} — {alert.service}", style=color)
    body = Text(f"Rule: {alert.rule_name}\n{alert.message}", style="white")
    console.print(Panel(body, title=title, border_style=color))


async def _post_slack(webhook_url: str, alert: Alert) -> None:
    if not webhook_url:
        return
    state = "resolved" if alert.resolved_at else "firing"
    color_map = {"critical": "#FF0000", "warning": "#FFA500", "info": "#36a64f"}
    color = color_map.get(alert.severity.lower(), "#cccccc")
    payload = {
        "attachments": [
            {
                "color": color,
                "title": f"[{state.upper()}] {alert.severity.upper()} — {alert.service}",
                "text": f"*Rule:* {alert.rule_name}\n{alert.message}",
                "footer": "sre-guard",
                "ts": int(alert.fired_at.timestamp()),
            }
        ]
    }
    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(webhook_url, json=payload)
            if resp.status_code not in (200, 204):
                logger.warning(
                    "Slack webhook returned HTTP %s: %s",
                    resp.status_code,
                    resp.text[:200],
                )
    except httpx.RequestError as exc:
        logger.warning("Slack webhook request failed: %s", exc)


async def fire(alert: Alert, slack_webhook_url: str = "") -> None:
    """Log, print, and optionally post the alert to Slack."""
    logger.info("Alert fired: %s", alert)
    _log_to_file(alert)
    _print_rich(alert)
    webhook = slack_webhook_url or os.getenv("SLACK_WEBHOOK_URL", "")
    if webhook:
        await _post_slack(webhook, alert)


def read_alert_log(service: str, tail: int = 50) -> list[dict]:
    """Read the last *tail* alert entries for a service from the log file."""
    log_file = _LOG_DIR / f"{service}.jsonl"
    if not log_file.exists():
        return []
    lines: list[dict] = []
    try:
        with log_file.open() as fh:
            for line in fh:
                line = line.strip()
                if line:
                    try:
                        lines.append(json.loads(line))
                    except json.JSONDecodeError:
                        pass
    except OSError:
        return []
    return lines[-tail:]
