"""Worker-side round-trip for the chunk-13 `object` session-state backend.

The API exposes three internal endpoints under
`/v1/internal/runs/{run_id}/session-state/`:

    GET  …/manifest               -> {"entries":[{"key","size","mtime"}]}
    GET  …/objects/{rel}          -> 302 redirect to a presigned URL
    PUT  …/objects/{rel}          -> raw body, in-band write

This module wraps those endpoints. On worker startup the client
downloads every object under the per-session bucket prefix into a
local emptyDir at NS_SESSION_STATE_DIR; on finalize, it walks the dir
and PUTs everything back. The emptyDir is ephemeral — the API +
MinIO are the durable storage.

Per-file failures are logged-and-skipped; manifest fetch failure
yields an empty set so the worker falls back to a fresh SDK session
instead of crashing on a missing transcript.
"""

from __future__ import annotations

import logging
from pathlib import Path
from typing import Any

import httpx

logger = logging.getLogger("nightshift-worker-claude.session_state")

# Mirror the API-side cap (defaultMaxBytes in
# internal/api/sessionstate/service.go). Files larger than this are
# skipped on upload — uploading would 413 anyway.
MAX_UPLOAD_BYTES = 64 * 1024 * 1024


class SessionStateClient:
    """Async client for the API session-state endpoints. Reuses the
    same bearer headers as APIClient (`worker_credential` scoped to
    this run)."""

    def __init__(
        self,
        base_url: str,
        run_id: str,
        headers: dict[str, str],
        timeout: float = 120.0,
    ) -> None:
        self._base = f"{base_url.rstrip('/')}/v1/internal/runs/{run_id}/session-state"
        self._headers = dict(headers)
        self._timeout = timeout

    async def fetch_into(self, dest: Path) -> set[str]:
        """Download every object listed in the manifest into `dest`.

        Returns the set of relative keys that landed on disk. An empty
        set is the signal to skip SDK resume — either there's no prior
        session state, or the manifest fetch failed.
        """
        dest.mkdir(parents=True, exist_ok=True)
        landed: set[str] = set()

        async with httpx.AsyncClient(
            timeout=self._timeout,
            headers=self._headers,
            follow_redirects=True,
        ) as client:
            try:
                resp = await client.get(f"{self._base}/manifest")
            except httpx.HTTPError as e:
                logger.warning("session-state: manifest fetch failed: %s", e)
                return landed
            if resp.status_code != 200:
                logger.warning(
                    "session-state: manifest status=%d body=%s",
                    resp.status_code,
                    resp.text[:200],
                )
                return landed

            try:
                entries = (resp.json() or {}).get("entries") or []
            except ValueError:
                logger.warning("session-state: manifest decode failed")
                return landed

            for entry in entries:
                key = entry.get("key", "")
                if not key:
                    continue
                target = dest / key
                target.parent.mkdir(parents=True, exist_ok=True)
                url = f"{self._base}/objects/{key}"
                try:
                    async with client.stream("GET", url) as r:
                        if r.status_code != 200:
                            logger.warning(
                                "session-state: GET %s status=%d",
                                key,
                                r.status_code,
                            )
                            continue
                        with open(target, "wb") as f:
                            async for chunk in r.aiter_bytes(64 * 1024):
                                f.write(chunk)
                    landed.add(key)
                except httpx.HTTPError as e:
                    logger.warning("session-state: GET %s failed: %s", key, e)
                    continue

        if landed:
            logger.info("session-state: fetched %d object(s)", len(landed))
        return landed

    async def upload_from(self, src: Path) -> tuple[int, int]:
        """PUT every regular file under `src` to its corresponding key.

        Returns (uploaded, failed). Best-effort: per-file errors are
        logged-and-skipped, never raised.
        """
        uploaded = 0
        failed = 0
        if not src.is_dir():
            return (0, 0)

        async with httpx.AsyncClient(
            timeout=self._timeout, headers=self._headers
        ) as client:
            for path in _walk_files(src):
                rel = path.relative_to(src).as_posix()
                if not rel:
                    continue
                size = path.stat().st_size
                if size > MAX_UPLOAD_BYTES:
                    logger.warning(
                        "session-state: skip %s (%d bytes > cap %d)",
                        rel,
                        size,
                        MAX_UPLOAD_BYTES,
                    )
                    failed += 1
                    continue
                try:
                    body = path.read_bytes()
                except OSError as e:
                    logger.warning("session-state: read %s failed: %s", rel, e)
                    failed += 1
                    continue
                url = f"{self._base}/objects/{rel}"
                try:
                    resp = await client.put(
                        url,
                        content=body,
                        headers={"Content-Type": _guess_content_type(rel)},
                    )
                except httpx.HTTPError as e:
                    logger.warning("session-state: PUT %s failed: %s", rel, e)
                    failed += 1
                    continue
                if resp.status_code >= 400:
                    logger.warning(
                        "session-state: PUT %s status=%d body=%s",
                        rel,
                        resp.status_code,
                        resp.text[:200],
                    )
                    failed += 1
                    continue
                uploaded += 1

        return (uploaded, failed)


def _walk_files(root: Path) -> list[Path]:
    """Return every regular file under `root`, sorted for determinism.
    Symlinks are skipped — the SDK doesn't write them and we don't
    want to follow into surprising places."""
    out: list[Path] = []
    for p in sorted(root.rglob("*")):
        if p.is_symlink():
            continue
        if p.is_file():
            out.append(p)
    return out


def _guess_content_type(rel: str) -> str:
    """Cheap content-type heuristic. The API doesn't care — it stores
    whatever we send — but keeping JSONL identifiable helps when
    debugging via `mc cp`."""
    if rel.endswith(".jsonl") or rel.endswith(".json"):
        return "application/json"
    return "application/octet-stream"


__all__: list[Any] = ["SessionStateClient", "MAX_UPLOAD_BYTES"]
