"""Monitor — Prometheus query loop, k8s event watcher, and health checks."""

from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timezone
from typing import Optional

import httpx

from .alerter import Alert, fire
from .config import AlertRule, ServiceWatch, SREGuardConfig

logger = logging.getLogger(__name__)


class AlertState:
    """Tracks when an alert rule first became true, to implement for_duration."""

    def __init__(self) -> None:
        # rule_name -> datetime when condition first became true
        self._pending: dict[str, datetime] = {}
        # rule_name -> active Alert object (currently firing)
        self._active: dict[str, Alert] = {}

    def get_active(self) -> dict[str, Alert]:
        return dict(self._active)

    def condition_true(self, rule: AlertRule, service: str, value: float) -> bool:
        """Call when condition is true. Returns True if alert should now fire."""
        now = datetime.now(tz=timezone.utc)
        if rule.name not in self._pending:
            self._pending[rule.name] = now
        elapsed = (now - self._pending[rule.name]).total_seconds()
        if elapsed >= rule.for_duration and rule.name not in self._active:
            msg = (
                f"Metric value {value:.4f} {rule.comparison} threshold {rule.threshold} "
                f"for >{rule.for_duration}s. Query: {rule.query}"
            )
            self._active[rule.name] = Alert(
                service=service,
                rule_name=rule.name,
                severity=rule.severity,
                message=msg,
            )
            return True
        return False

    def condition_false(self, rule: AlertRule) -> Optional[Alert]:
        """Call when condition is false. Returns resolved Alert if it was active."""
        self._pending.pop(rule.name, None)
        if rule.name in self._active:
            alert = self._active.pop(rule.name)
            alert.resolved_at = datetime.now(tz=timezone.utc)
            return alert
        return None


class MonitorLoop:
    """Main polling loop — checks health, Prometheus metrics, fires alerts."""

    def __init__(self, config: SREGuardConfig) -> None:
        self.config = config
        self._alert_states: dict[str, AlertState] = {}  # service name -> state
        self._silences: dict[str, datetime] = {}  # service name -> silence_until
        self._running = False

    def _get_state(self, service: str) -> AlertState:
        if service not in self._alert_states:
            self._alert_states[service] = AlertState()
        return self._alert_states[service]

    def silence(self, service: str, until: datetime) -> None:
        self._silences[service] = until

    def is_silenced(self, service: str) -> bool:
        until = self._silences.get(service)
        if until is None:
            return False
        if datetime.now(tz=timezone.utc) >= until:
            del self._silences[service]
            return False
        return True

    def current_alerts(self) -> dict[str, list[dict]]:
        """Return all currently active alerts keyed by service name."""
        result: dict[str, list[dict]] = {}
        for svc, state in self._alert_states.items():
            active = state.get_active()
            if active:
                result[svc] = [a.to_dict() for a in active.values()]
        return result

    async def tick(self) -> None:
        """One poll cycle — iterate all services concurrently."""
        tasks = [
            self._check_service(svc) for svc in self.config.services
        ]
        results = await asyncio.gather(*tasks, return_exceptions=True)
        for svc, exc in zip(self.config.services, results):
            if isinstance(exc, Exception):
                logger.error("Unhandled error checking service %s: %s", svc.name, exc)

    async def run(self) -> None:
        """Run the polling loop until cancelled."""
        self._running = True
        logger.info(
            "MonitorLoop started — polling every %ds for %d service(s)",
            self.config.poll_interval_seconds,
            len(self.config.services),
        )
        try:
            while self._running:
                await self.tick()
                await asyncio.sleep(self.config.poll_interval_seconds)
        except asyncio.CancelledError:
            logger.info("MonitorLoop cancelled, stopping.")
            self._running = False
            raise

    async def _check_service(self, svc: ServiceWatch) -> None:
        state = self._get_state(svc.name)
        silenced = self.is_silenced(svc.name)

        # Health check
        if svc.health_url:
            await self._health_check(svc, state, silenced)

        # Prometheus alert rules
        for rule in svc.alert_rules:
            await self._check_rule(svc, rule, state, silenced)

    async def _health_check(
        self, svc: ServiceWatch, state: AlertState, silenced: bool
    ) -> None:
        health_rule = AlertRule(
            name="HealthCheckFailed",
            query=f"GET {svc.health_url}",
            threshold=1.0,
            comparison="lt",
            severity="critical",
            for_duration=30,
        )
        try:
            async with httpx.AsyncClient(timeout=10.0) as client:
                resp = await client.get(svc.health_url)
            is_healthy = resp.status_code < 400
        except httpx.RequestError as exc:
            logger.warning("Health check failed for %s: %s", svc.name, exc)
            is_healthy = False

        if not is_healthy:
            should_fire = state.condition_true(health_rule, svc.name, 0.0)
            if should_fire and not silenced:
                alert = state.get_active().get("HealthCheckFailed")
                if alert:
                    await fire(alert, self.config.slack_webhook_url)
        else:
            resolved = state.condition_false(health_rule)
            if resolved and not silenced:
                await fire(resolved, self.config.slack_webhook_url)

    async def _check_rule(
        self,
        svc: ServiceWatch,
        rule: AlertRule,
        state: AlertState,
        silenced: bool,
    ) -> None:
        value = await self._query_prometheus(svc.prometheus_url, rule.query)
        if value is None:
            logger.debug(
                "No result from Prometheus for rule %s on service %s",
                rule.name,
                svc.name,
            )
            return

        if rule.evaluate(value):
            should_fire = state.condition_true(rule, svc.name, value)
            if should_fire and not silenced:
                alert = state.get_active().get(rule.name)
                if alert:
                    await fire(alert, self.config.slack_webhook_url)
        else:
            resolved = state.condition_false(rule)
            if resolved and not silenced:
                await fire(resolved, self.config.slack_webhook_url)

    async def _query_prometheus(self, prometheus_url: str, query: str) -> Optional[float]:
        """Execute a PromQL instant query; return the first scalar result or None."""
        url = prometheus_url.rstrip("/") + "/api/v1/query"
        try:
            async with httpx.AsyncClient(timeout=10.0) as client:
                resp = await client.get(url, params={"query": query})
            if resp.status_code != 200:
                logger.debug(
                    "Prometheus returned HTTP %s for query %r", resp.status_code, query
                )
                return None
            data = resp.json()
            results = data.get("data", {}).get("result", [])
            if not results:
                return None
            # Take the first result's value
            raw = results[0].get("value", [None, None])[1]
            return float(raw) if raw is not None else None
        except (httpx.RequestError, ValueError, KeyError) as exc:
            logger.debug("Prometheus query error for %r: %s", query, exc)
            return None
