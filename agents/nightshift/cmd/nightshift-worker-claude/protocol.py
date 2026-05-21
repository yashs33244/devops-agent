"""HTTPX wrapper for the Workers inner-surface RPCs.

The reference Python worker speaks to the API via grpc-gateway-proxied
JSON, not directly over gRPC. This keeps the dependency footprint small
(no protobuf/grpcio in the image) and matches cr0n's worker.py shape
1:1, just rebased onto nightshift's URL layout.

URL shape (every call carries Authorization: Bearer NS_WORKER_CREDENTIAL):

  POST {base}/v1/internal/runs/{run_id}/events
  POST {base}/v1/internal/runs/{run_id}:complete
  POST {base}/v1/internal/runs/{run_id}:fail
  GET  {base}/v1/internal/runs/{run_id}/cancellation
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any

import httpx

logger = logging.getLogger("nightshift-worker-claude.protocol")


class APIClient:
    """Async HTTPX client for the Workers inner surface."""

    def __init__(
        self,
        base_url: str,
        run_id: str,
        worker_credential: str,
        timeout: float = 30.0,
    ) -> None:
        self._base = base_url.rstrip("/")
        self._run_id = run_id
        self._headers = {"Authorization": f"Bearer {worker_credential}"}
        self._http = httpx.AsyncClient(timeout=timeout, headers=self._headers)

    async def aclose(self) -> None:
        await self._http.aclose()

    async def __aenter__(self) -> "APIClient":
        return self

    async def __aexit__(self, *exc: object) -> None:
        await self.aclose()

    @property
    def headers(self) -> dict[str, str]:
        """Auth headers — exposed so artifact_tools can reuse them on
        their own httpx.AsyncClient instances when posting to the API."""
        return dict(self._headers)

    @property
    def base_url(self) -> str:
        return self._base

    def _runs_url(self, suffix: str) -> str:
        return f"{self._base}/v1/internal/runs/{self._run_id}{suffix}"

    async def emit(self, type_: str, raw: Any) -> None:
        """POST an event. The server assigns `index`; the worker only
        sets type, timestamp, and raw."""
        body = {
            "event": {
                "type": type_,
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "raw": raw,
            },
        }
        try:
            resp = await self._http.post(self._runs_url("/events"), json=body)
            if resp.status_code >= 400:
                logger.warning(
                    "emit type=%s failed: %d %s",
                    type_,
                    resp.status_code,
                    resp.text[:200],
                )
        except httpx.HTTPError as e:
            logger.warning("emit type=%s failed: %s", type_, e)

    async def poll_cancellation(self) -> bool:
        """GET cancellation flag. Returns False on any error
        (worker continues — cancellation is best-effort)."""
        try:
            resp = await self._http.get(self._runs_url("/cancellation"))
            if resp.status_code == 200:
                return bool(resp.json().get("cancelled", False))
        except httpx.HTTPError:
            pass
        return False

    async def complete(
        self,
        sdk_session_id: str,
        usage: dict[str, Any] | None = None,
    ) -> None:
        """POST :complete. The session_id field carries the SDK-internal
        session id by design (workers.md §4 — inner surface MAY carry
        it). The platform's session_id is preserved on the Run record;
        the API stores this value as a Record attribute for resume."""
        body: dict[str, Any] = {"session_id": sdk_session_id}
        if usage:
            body["usage"] = usage
        resp = await self._http.post(self._runs_url(":complete"), json=body)
        if resp.status_code >= 400:
            raise RuntimeError(
                f"CompleteRun failed: {resp.status_code} {resp.text[:200]}"
            )

    async def fail(self, error: str) -> None:
        """POST :fail."""
        resp = await self._http.post(
            self._runs_url(":fail"),
            json={"error": error},
        )
        if resp.status_code >= 400:
            logger.error(
                "FailRun failed: %d %s", resp.status_code, resp.text[:200]
            )

    async def list_session_attachments(
        self, session_id: str, owner_id: str
    ) -> list[dict[str, Any]]:
        """User uploads attached to a chat thread. Returns [] on failure."""
        if not session_id or not owner_id:
            return []
        try:
            resp = await self._http.get(
                f"{self._base}/v1/artifacts",
                params={"session_id": session_id, "owner_id": owner_id},
            )
            if resp.status_code != 200:
                logger.warning(
                    "ListArtifacts(session_id=%s) failed: %d %s",
                    session_id,
                    resp.status_code,
                    resp.text[:200],
                )
                return []
            arts = resp.json().get("artifacts") or []
            # Filter out worker-produced artifacts. Load-bearing now
            # that agent outputs share the same session scope as user
            # uploads (PR-A stamps session_id on both).
            return [a for a in arts if not (a.get("runId") or a.get("run_id"))]
        except httpx.HTTPError as e:
            logger.warning("ListArtifacts(session_id=%s) failed: %s", session_id, e)
            return []

    async def get_user_config(self, user_id: str) -> dict[str, Any] | None:
        """GET /v1/users/{user_id}/config — Config Dispenser."""
        try:
            resp = await self._http.get(
                f"{self._base}/v1/users/{user_id}/config",
            )
            if resp.status_code == 200:
                # The gateway wraps the proto response as {config: {...}}.
                payload = resp.json()
                return payload.get("config") or payload
            logger.warning(
                "GetUserConfig failed: %d %s",
                resp.status_code,
                resp.text[:200],
            )
        except httpx.HTTPError as e:
            logger.warning("GetUserConfig failed: %s", e)
        return None
