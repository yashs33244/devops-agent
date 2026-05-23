"""Pydantic config model — loads sre-guard.yaml and environment variables."""

from __future__ import annotations

import os
from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, Field, model_validator


class AlertRule(BaseModel):
    name: str
    query: str
    threshold: float
    comparison: Literal["gt", "lt", "eq"] = "gt"
    severity: Literal["critical", "warning", "info"] = "warning"
    for_duration: int = 60  # seconds before firing

    def evaluate(self, value: float) -> bool:
        """Return True if the rule condition is met (alert should fire)."""
        if self.comparison == "gt":
            return value > self.threshold
        if self.comparison == "lt":
            return value < self.threshold
        if self.comparison == "eq":
            return abs(value - self.threshold) < 1e-9
        return False


class ServiceWatch(BaseModel):
    name: str
    prometheus_url: str = "http://prometheus:9090"
    namespace: str = "default"
    health_url: str = ""
    alert_rules: list[AlertRule] = Field(default_factory=list)


class SREGuardConfig(BaseModel):
    poll_interval_seconds: int = 30
    api_port: int = 8888
    log_level: str = "INFO"
    slack_webhook_url: str = ""
    services: list[ServiceWatch] = Field(default_factory=list)

    @model_validator(mode="after")
    def _apply_env_overrides(self) -> "SREGuardConfig":
        webhook = os.getenv("SLACK_WEBHOOK_URL", "").strip()
        if webhook:
            self.slack_webhook_url = webhook
        return self


_DEFAULT_CONFIG_PATHS = [
    Path("config/sre-guard.yaml"),
    Path("/etc/sre-guard/sre-guard.yaml"),
    Path.home() / ".config" / "sre-guard" / "sre-guard.yaml",
]


def load_config(path: str | Path | None = None) -> SREGuardConfig:
    """Load config from a YAML file, falling back to defaults."""
    candidates: list[Path] = []
    if path:
        candidates.append(Path(path))
    candidates.extend(_DEFAULT_CONFIG_PATHS)

    for candidate in candidates:
        if candidate.exists():
            with candidate.open() as fh:
                data = yaml.safe_load(fh) or {}
            return SREGuardConfig(**data)

    return SREGuardConfig()
