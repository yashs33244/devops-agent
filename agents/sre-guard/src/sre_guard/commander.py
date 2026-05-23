"""Commander — FastAPI REST API for receiving commands on port 8888."""

from __future__ import annotations

import logging
from datetime import datetime, timedelta, timezone
from typing import Optional

from fastapi import FastAPI, HTTPException, status
from pydantic import BaseModel

from .config import AlertRule, ServiceWatch, SREGuardConfig
from .investigator import diagnose
from .monitor import MonitorLoop

logger = logging.getLogger(__name__)

app = FastAPI(
    title="SRE Guard",
    description="Continuous service monitoring daemon — command interface",
    version="0.1.0",
)

# These are injected at startup by daemon.py
_monitor: Optional[MonitorLoop] = None
_config: Optional[SREGuardConfig] = None


def bind(monitor: MonitorLoop, config: SREGuardConfig) -> None:
    """Bind the shared MonitorLoop and config to this API module."""
    global _monitor, _config
    _monitor = monitor
    _config = config


def _require_monitor() -> MonitorLoop:
    if _monitor is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Monitor not yet initialised.",
        )
    return _monitor


def _require_config() -> SREGuardConfig:
    if _config is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Config not yet initialised.",
        )
    return _config


# ---------------------------------------------------------------------------
# Request / response models
# ---------------------------------------------------------------------------


class WatchRequest(BaseModel):
    service: str
    prometheus_url: str = "http://prometheus:9090"
    namespace: str = "default"
    health_url: str = ""
    alert_rules: list[AlertRule] = []


class SilenceRequest(BaseModel):
    minutes: int = 30


class DiagnoseRequest(BaseModel):
    context: str = ""


class RunbookAction(BaseModel):
    action: str  # e.g. "restart_pod", "scale_up"
    params: dict = {}


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------


@app.get("/health")
async def health() -> dict:
    return {"status": "ok"}


@app.get("/status")
async def get_status() -> dict:
    monitor = _require_monitor()
    config = _require_config()

    active_alerts = monitor.current_alerts()
    services_out = []
    for svc in config.services:
        silenced = monitor.is_silenced(svc.name)
        silence_until = None
        if silenced:
            silence_until = monitor._silences.get(svc.name)
            if silence_until:
                silence_until = silence_until.isoformat()
        services_out.append(
            {
                "name": svc.name,
                "namespace": svc.namespace,
                "prometheus_url": svc.prometheus_url,
                "health_url": svc.health_url,
                "alert_rules": len(svc.alert_rules),
                "active_alerts": active_alerts.get(svc.name, []),
                "silenced": silenced,
                "silence_until": silence_until,
            }
        )
    return {
        "daemon": "running",
        "poll_interval_seconds": config.poll_interval_seconds,
        "services": services_out,
        "total_active_alerts": sum(len(v) for v in active_alerts.values()),
    }


@app.post("/watch", status_code=status.HTTP_201_CREATED)
async def add_watch(req: WatchRequest) -> dict:
    config = _require_config()
    # Deduplicate by service name
    existing = next((s for s in config.services if s.name == req.service), None)
    if existing:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=f"Service '{req.service}' is already being watched.",
        )
    svc = ServiceWatch(
        name=req.service,
        prometheus_url=req.prometheus_url,
        namespace=req.namespace,
        health_url=req.health_url,
        alert_rules=req.alert_rules,
    )
    config.services.append(svc)
    logger.info("Added service watch: %s", req.service)
    return {"added": req.service}


@app.delete("/watch/{service}")
async def remove_watch(service: str) -> dict:
    config = _require_config()
    before = len(config.services)
    config.services = [s for s in config.services if s.name != service]
    if len(config.services) == before:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Service '{service}' not found in watch list.",
        )
    logger.info("Removed service watch: %s", service)
    return {"removed": service}


@app.post("/diagnose/{service}")
async def diagnose_service(service: str, req: DiagnoseRequest = DiagnoseRequest()) -> dict:
    config = _require_config()
    monitor = _require_monitor()

    svc = next((s for s in config.services if s.name == service), None)
    if not svc:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Service '{service}' not found.",
        )

    # Build context from active alerts if no explicit context provided
    context = req.context
    if not context:
        active = monitor.current_alerts().get(service, [])
        if active:
            context = "; ".join(
                f"{a.get('rule_name', '')}: {a.get('message', '')}" for a in active
            )
        else:
            context = f"Ad-hoc investigation of service '{service}' — no active alerts."

    findings = await diagnose(service, context)
    return {"service": service, "findings": findings}


@app.post("/silence/{service}")
async def silence_service(service: str, req: SilenceRequest = SilenceRequest()) -> dict:
    config = _require_config()
    monitor = _require_monitor()

    svc = next((s for s in config.services if s.name == service), None)
    if not svc:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Service '{service}' not found.",
        )

    until = datetime.now(tz=timezone.utc) + timedelta(minutes=req.minutes)
    monitor.silence(service, until)
    logger.info("Silenced alerts for %s until %s", service, until.isoformat())
    return {"silenced": service, "until": until.isoformat(), "minutes": req.minutes}


@app.post("/runbook/{service}")
async def run_runbook(service: str, action: RunbookAction) -> dict:
    config = _require_config()
    svc = next((s for s in config.services if s.name == service), None)
    if not svc:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Service '{service}' not found.",
        )

    result = await _execute_runbook_action(service, svc.namespace, action.action, action.params)
    return {"service": service, "action": action.action, "result": result}


async def _execute_runbook_action(
    service: str, namespace: str, action: str, params: dict
) -> str:
    """Execute a predefined runbook action via kubectl subprocess."""
    import asyncio
    import shutil

    kubectl = shutil.which("kubectl")
    if not kubectl:
        return "kubectl not found — cannot execute runbook actions."

    if action == "restart_pod":
        label = params.get("label", f"app={service}")
        cmd = [kubectl, "rollout", "restart", f"deployment/{service}", "-n", namespace]
    elif action == "scale_up":
        replicas = str(params.get("replicas", 3))
        cmd = [kubectl, "scale", f"deployment/{service}", f"--replicas={replicas}", "-n", namespace]
    elif action == "describe_pods":
        cmd = [kubectl, "describe", "pods", "-n", namespace, "-l", f"app={service}"]
    elif action == "get_events":
        cmd = [kubectl, "get", "events", "-n", namespace, "--sort-by=.lastTimestamp", "--field-selector", f"involvedObject.name={service}"]
    else:
        return f"Unknown runbook action: '{action}'. Supported: restart_pod, scale_up, describe_pods, get_events."

    try:
        proc = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await asyncio.wait_for(proc.communicate(), timeout=30.0)
        out = stdout.decode("utf-8", errors="replace").strip()
        err = stderr.decode("utf-8", errors="replace").strip()
        if proc.returncode != 0:
            return f"Command failed (code {proc.returncode}): {err}"
        return out or "Command completed with no output."
    except asyncio.TimeoutError:
        return "Runbook action timed out after 30s."
    except OSError as exc:
        return f"Failed to execute runbook: {exc}"
