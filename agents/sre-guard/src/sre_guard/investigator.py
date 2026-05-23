"""Investigator — AI-powered diagnosis via holmesgpt CLI subprocess."""

from __future__ import annotations

import asyncio
import json
import logging
import shutil
from typing import Optional

logger = logging.getLogger(__name__)

_HOLMES_BINARY = "holmes"


def _find_holmes() -> Optional[str]:
    """Locate the holmesgpt binary; return None if not installed."""
    return shutil.which(_HOLMES_BINARY)


async def diagnose(service: str, context: str) -> str:
    """
    Invoke holmesgpt to investigate a service incident.

    If holmesgpt is not installed, fall back to a structured analysis built
    from the provided context string.
    """
    binary = _find_holmes()
    if binary:
        return await _diagnose_with_holmes(binary, service, context)
    logger.warning(
        "holmesgpt binary '%s' not found — using fallback analysis", _HOLMES_BINARY
    )
    return _fallback_analysis(service, context)


async def _diagnose_with_holmes(binary: str, service: str, context: str) -> str:
    prompt = f"investigate {service}: {context}"
    cmd = [binary, "ask", prompt, "--output", "json"]
    logger.info("Running holmesgpt: %s", " ".join(cmd))
    try:
        proc = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            stdout, stderr = await asyncio.wait_for(proc.communicate(), timeout=120.0)
        except asyncio.TimeoutError:
            proc.kill()
            return (
                f"[ERROR] holmesgpt timed out after 120s investigating service '{service}'."
            )

        raw = stdout.decode("utf-8", errors="replace").strip()
        err = stderr.decode("utf-8", errors="replace").strip()

        if proc.returncode != 0:
            logger.warning(
                "holmesgpt exited with code %d. stderr: %s", proc.returncode, err[:500]
            )
            if raw:
                return f"holmesgpt exited {proc.returncode}. Output:\n{raw}"
            return (
                f"[ERROR] holmesgpt failed (code {proc.returncode}): {err[:500]}"
            )

        # holmesgpt may or may not produce valid JSON even with --output json
        try:
            parsed = json.loads(raw)
            # Normalise: extract common keys
            findings = (
                parsed.get("findings")
                or parsed.get("analysis")
                or parsed.get("result")
                or raw
            )
            if isinstance(findings, dict):
                return json.dumps(findings, indent=2)
            return str(findings)
        except json.JSONDecodeError:
            # Return raw text if JSON parsing fails
            return raw or "holmesgpt returned no output."

    except FileNotFoundError:
        return f"[ERROR] holmesgpt binary not found at '{binary}'."
    except OSError as exc:
        return f"[ERROR] Failed to execute holmesgpt: {exc}"


def _fallback_analysis(service: str, context: str) -> str:
    """
    Structured heuristic analysis when holmesgpt is unavailable.
    Parses the context string for known alert signals and produces recommendations.
    """
    lines = context.strip().splitlines()
    findings: list[str] = []
    recommendations: list[str] = []

    context_lower = context.lower()

    if "errorrate" in context_lower or "5xx" in context_lower or "error rate" in context_lower:
        findings.append("Elevated error rate detected (5xx responses above threshold).")
        recommendations += [
            "Check recent deployments — roll back if correlated with error spike.",
            "Inspect application logs: kubectl logs -n <namespace> -l app=" + service + " --tail=200",
            "Verify downstream dependencies (databases, caches, external APIs).",
        ]

    if "latency" in context_lower or "p95" in context_lower or "duration" in context_lower:
        findings.append("High latency detected (P95 response time above threshold).")
        recommendations += [
            "Check CPU/memory pressure: kubectl top pods -n <namespace>",
            "Look for slow DB queries or connection pool exhaustion.",
            "Consider enabling connection keep-alive or increasing replicas.",
        ]

    if "poddown" in context_lower or "pod down" in context_lower or "up{" in context_lower:
        findings.append("Pod(s) reporting as down — Prometheus 'up' metric < 1.")
        recommendations += [
            "Check pod status: kubectl get pods -n <namespace> -l app=" + service,
            "Inspect recent events: kubectl describe pod -n <namespace> <pod-name>",
            "Review OOMKilled or CrashLoopBackOff: kubectl get events -n <namespace> --sort-by='.lastTimestamp'",
        ]

    if "healthcheck" in context_lower or "health check" in context_lower:
        findings.append("Health endpoint is unreachable or returning non-2xx.")
        recommendations += [
            "Verify the service is running: kubectl get svc,pods -n <namespace>",
            "Test directly: kubectl port-forward svc/" + service + " 8080:8080 -n <namespace>",
            "Check readiness/liveness probe configuration in the Helm values.",
        ]

    if not findings:
        findings.append(f"Generic alert fired for service '{service}'.")
        recommendations.append("Review metrics in Prometheus/Grafana for context.")
        recommendations.append("Check pod logs and recent events in the namespace.")

    result = {
        "service": service,
        "diagnosis_source": "sre-guard-fallback",
        "context": context[:500],
        "findings": findings,
        "recommendations": recommendations,
    }
    return json.dumps(result, indent=2)
