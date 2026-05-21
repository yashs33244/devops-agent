"""Standalone worker entrypoint for K8s Job execution.

Runs a Claude agent and POSTs events back to the nightshift-api via
grpc-gateway. Configured entirely via NS_* environment variables (see
protos/nightshift/v1/worker-protocol.md).

Direct port of cr0n-a's worker.py with these adaptations:
  - HTTPX endpoints rebased onto nightshift's grpc-gateway paths
    (/v1/internal/runs/{run_id}/events, :complete, :fail, /cancellation,
    /v1/users/{user_id}/config).
  - X-Worker-Secret → Authorization: Bearer <NS_WORKER_CREDENTIAL>
    (HMAC-signed, scoped to RUN_ID per chunk 8c).
  - cr0n's RUN_CLAUDE_SESSION_ID env → NS_SDK_SESSION_ID. The platform
    looks up the prior run's SDK session id via `attrSDKSessionID`
    Record attribute and injects it on resume; the worker never crosses
    the SDK id over the outer surface.
  - SDK transcript persistence: when NS_SESSION_STATE_DIR is set
    (chunk-13 mount), symlink <workspace>/.claude/projects to it so the
    Claude SDK's JSONL files persist across runs of the same session.
"""

from __future__ import annotations

import logging
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

import httpx
from claude_agent_sdk import (
    AgentDefinition,
    ClaudeAgentOptions,
    ClaudeSDKClient,
    ResultMessage,
)

from protocol import APIClient
from serialization import message_type, serialize_message
from session_state import SessionStateClient

logging.basicConfig(
    level=logging.INFO, format="[ns-worker-claude] %(levelname)s %(message)s"
)
logger = logging.getLogger("nightshift-worker-claude")

# Required configuration
RUN_ID = os.environ["NS_RUN_ID"]
RUN_PROMPT = os.environ["NS_PROMPT"]
API_CALLBACK_URL = os.environ["NS_API_URL"]
WORKER_CREDENTIAL = os.environ["NS_WORKER_CREDENTIAL"]

# Optional configuration
USER_ID = os.getenv("NS_USER_ID", "")
SESSION_ID = os.getenv("NS_SESSION_ID", "")
SDK_SESSION_ID = os.getenv("NS_SDK_SESSION_ID", "")
SESSION_STATE_DIR = os.getenv("NS_SESSION_STATE_DIR", "")
SESSION_STATE_BACKEND = os.getenv("NS_SESSION_STATE_BACKEND", "")
WORKSPACE = os.getenv("NS_WORKSPACE", "/home/nightshift/workspace")

# OpenBao integration (mirrors cr0n's worker; fetches ANTHROPIC_API_KEY).
BAO_URL = os.getenv("NS_OPENBAO_ADDR", "http://openbao.nightshift.svc:8200")
BAO_ROLE = os.getenv("NS_OPENBAO_AUTH_ROLE", "nightshift-worker")
BAO_API_KEY_PATH = os.getenv(
    "NS_ANTHROPIC_KEY_PATH", "nightshift/anthropic-api-key"
)

# Cancellation polling — every N events.
CANCEL_POLL_INTERVAL = int(os.getenv("NS_CANCEL_POLL_INTERVAL", "5"))


async def _fetch_anthropic_key_from_bao() -> None:
    """Authenticate to OpenBao via K8s SA and fetch the Anthropic API key
    into ANTHROPIC_API_KEY for the SDK to pick up. Mirrors cr0n's
    `_fetch_secrets_from_bao`."""
    from bao_client import BaoClient

    bao = BaoClient(BAO_URL)
    await bao.start()
    try:
        if not await bao.login_kubernetes(role=BAO_ROLE):
            logger.error("openbao auth failed — ANTHROPIC_API_KEY not set")
            return
        kv = await bao.kv_read(BAO_API_KEY_PATH)
        if kv:
            api_key = kv.get("api-key") or kv.get("api_key") or ""
            if api_key:
                os.environ["ANTHROPIC_API_KEY"] = api_key
                logger.info("fetched ANTHROPIC_API_KEY from openbao")
            else:
                logger.warning(
                    "openbao kv at %s missing api-key field", BAO_API_KEY_PATH
                )
        else:
            logger.warning(
                "openbao kv read failed for %s", BAO_API_KEY_PATH
            )
    finally:
        await bao.close()


def _link_session_state_dir() -> None:
    """Symlink ~/.claude/projects → NS_SESSION_STATE_DIR so the SDK's
    JSONL transcripts land on the per-session persistent volume.
    No-op when NS_SESSION_STATE_DIR is unset (backend=none).

    The Claude Agent SDK writes transcripts to
    ``~/.claude/projects/<sanitized-cwd>/<sessionId>.jsonl`` —
    HOME-based, NOT under cwd. The chunk-9 hardening mounts
    /home/nightshift as an emptyDir per pod, so without this symlink
    every pod starts with a fresh empty ~/.claude tree and the SDK
    can never resume a prior session.
    """
    if not SESSION_STATE_DIR:
        return
    state_root = Path(SESSION_STATE_DIR)
    state_root.mkdir(parents=True, exist_ok=True)
    claude_dir = Path.home() / ".claude"
    claude_dir.mkdir(parents=True, exist_ok=True)
    target = claude_dir / "projects"
    if target.is_symlink() or target.exists():
        # Pre-existing target (rare in a fresh container, but possible
        # if the image bakes a placeholder). Replace with the link.
        if target.is_symlink():
            target.unlink()
        elif target.is_dir():
            # Move existing contents into the persistent dir, then link.
            for p in target.iterdir():
                p.rename(state_root / p.name)
            target.rmdir()
        else:
            target.unlink()
    target.symlink_to(state_root, target_is_directory=True)
    logger.info("linked %s -> %s", target, state_root)


async def _describe_session_attachments(api: APIClient) -> list[dict]:
    """List metadata (id, name, size_bytes, content_type) for the
    session's user-uploaded artifacts. Bytes are fetched lazily on
    demand by the `download_artifact` MCP tool."""
    if not SESSION_ID or not USER_ID:
        await api.emit(
            "system.attachments_skipped",
            {"reason": "missing session_id or user_id"},
        )
        return []
    arts = await api.list_session_attachments(SESSION_ID, USER_ID)
    meta = [
        {
            "id": a.get("id") or "",
            "name": a.get("name") or "",
            "size_bytes": a.get("sizeBytes") or a.get("size_bytes") or 0,
            "content_type": a.get("contentType") or a.get("content_type") or "",
        }
        for a in arts
        if a.get("id")
    ]
    if not meta:
        await api.emit(
            "system.attachments_none",
            {"session_id": SESSION_ID, "user_id": USER_ID},
        )
        return []
    total = sum(int(m["size_bytes"] or 0) for m in meta)
    logger.info(
        "described %d attachments (total=%d bytes): %s",
        len(meta),
        total,
        [(m["name"], m["size_bytes"]) for m in meta],
    )
    await api.emit(
        "system.attachments_described",
        {"count": len(meta), "total_bytes": total, "attachments": meta},
    )
    return meta


async def run() -> None:
    await _fetch_anthropic_key_from_bao()
    # The launcher's chunk-9 hardening sets readOnlyRootFilesystem=true
    # and mounts an emptyDir on /home/nightshift to give the CLI a
    # writable HOME. That overlay hides the Dockerfile-baked
    # /home/nightshift/workspace, so the SDK's `cwd=WORKSPACE` would
    # fail with FileNotFoundError. mkdir -p on every startup.
    Path(WORKSPACE).mkdir(parents=True, exist_ok=True)
    _link_session_state_dir()

    # Resume-target is potentially mutated below if the object backend
    # confirms the prior transcript is missing. Local copy keeps the
    # module-level constant unchanged for log clarity.
    sdk_session_id = SDK_SESSION_ID

    async with APIClient(
        base_url=API_CALLBACK_URL,
        run_id=RUN_ID,
        worker_credential=WORKER_CREDENTIAL,
    ) as api:
        # chunk-14 round-trip: pull prior session state out of the API
        # before the SDK opens the transcript. The fetch is best-effort
        # — on failure we fall back to a fresh session rather than
        # crashing the run with `--resume <missing>`.
        ss_client: SessionStateClient | None = None
        if SESSION_STATE_BACKEND == "object" and SESSION_STATE_DIR:
            ss_client = SessionStateClient(
                base_url=API_CALLBACK_URL,
                run_id=RUN_ID,
                headers=api.headers,
            )
            fetched = await ss_client.fetch_into(Path(SESSION_STATE_DIR))
            # Manifest keys are relative to the bucket per-session prefix
            # and include the SDK's <sanitized-cwd>/ subdirectory, so we
            # match by suffix rather than full equality.
            target_suffix = f"{sdk_session_id}.jsonl" if sdk_session_id else ""
            has_resume_target = target_suffix and any(
                k == target_suffix or k.endswith("/" + target_suffix)
                for k in fetched
            )
            if sdk_session_id and not has_resume_target:
                logger.warning(
                    "session-state: resume target %s.jsonl not in object store; starting fresh",
                    sdk_session_id,
                )
                sdk_session_id = ""
        # Notify API we're starting.
        await api.emit(
            "system.worker_started",
            {
                "image": "nightshift-worker-claude",
                "version": "0.1.0",
                "ts": datetime.now(timezone.utc).isoformat(),
            },
        )

        # Fetch user config from Config Dispenser
        # (agents + skills + mcp_servers).
        agents: dict[str, AgentDefinition] = {}
        mcp_servers: dict = {}
        allowed_mcp_tools: list[str] = []
        disallowed_mcp_tools: list[str] = []
        has_skills = False

        if USER_ID:
            config = await api.get_user_config(USER_ID)
            if config:
                # Build programmatic agent definitions
                for a in config.get("agents", []) or []:
                    agents[a["name"]] = AgentDefinition(
                        description=a.get("description", ""),
                        prompt=a.get("prompt", ""),
                        tools=a.get("tools") or None,
                        model=a.get("model") or None,
                    )
                if agents:
                    logger.info(
                        "loaded %d agents from config", len(agents)
                    )

                # Hydrate skills to filesystem
                for s in config.get("skills", []) or []:
                    skill_dir = (
                        Path(WORKSPACE)
                        / ".claude"
                        / "skills"
                        / s["name"]
                    )
                    skill_dir.mkdir(parents=True, exist_ok=True)
                    (skill_dir / "SKILL.md").write_text(s.get("content", ""))
                    has_skills = True
                if has_skills:
                    logger.info(
                        "hydrated %d skills to workspace",
                        len(config.get("skills") or []),
                    )

                # MCP servers — proto McpServerConfig → SDK shape. The
                # SDK uses separate dataclasses per transport and
                # silently drops servers whose declared `type` doesn't
                # match the actual endpoint, so type MUST be lowercase
                # "http"/"sse", never the proto enum string.
                for name, spec in (config.get("mcpServers") or {}).items():
                    transport = (spec.get("transport") or "").upper()
                    url = spec.get("url") or ""
                    if transport == "MCP_TRANSPORT_SSE":
                        type_ = "sse"
                    elif transport == "MCP_TRANSPORT_HTTP":
                        type_ = "http"
                    else:
                        # Unspecified: /sse suffix → SSE, else HTTP.
                        type_ = "sse" if url.rstrip("/").endswith("/sse") else "http"
                    mcp_servers[name] = {
                        "type": type_,
                        "url": url,
                        "headers": spec.get("headers") or {},
                    }
                if mcp_servers:
                    logger.info(
                        "loaded %d MCP servers from config: %s",
                        len(mcp_servers),
                        ", ".join(sorted(mcp_servers.keys())),
                    )
                allowed_mcp_tools = list(config.get("allowedMcpTools") or [])
                disallowed_mcp_tools = list(config.get("disallowedMcpTools") or [])
                if disallowed_mcp_tools:
                    logger.info(
                        "disallowing %d MCP tools: %s",
                        len(disallowed_mcp_tools),
                        ", ".join(sorted(disallowed_mcp_tools)),
                    )

        # Build allowed tools — same defaults as cr0n.
        allowed_tools = [
            "Read",
            "Write",
            "Edit",
            "Bash",
            "Glob",
            "Grep",
            "Agent",
        ]
        if has_skills:
            allowed_tools.append("Skill")
        if allowed_mcp_tools:
            allowed_tools.extend(allowed_mcp_tools)

        # Artifact / schedule MCP tools — always available. NOTE: these
        # call /internal/artifacts/* and /internal/schedules/* on the API,
        # which are Unimplemented until chunks 15/16/17 land. Tools that
        # hit unimplemented endpoints surface as agent-visible errors and
        # the agent reports back to the user. See README.md.
        from artifact_tools import TOOL_SERVER_NAME, create_artifact_tools

        artifact_server, artifact_allowed = create_artifact_tools(
            api_base_url=API_CALLBACK_URL,
            run_id=RUN_ID,
            user_id=USER_ID,
            headers=api.headers,
            session_id=SESSION_ID,
        )
        mcp_servers[TOOL_SERVER_NAME] = artifact_server
        allowed_tools.extend(artifact_allowed)

        options = ClaudeAgentOptions(
            allowed_tools=allowed_tools,
            permission_mode="bypassPermissions",
            cwd=WORKSPACE,
            setting_sources=["project"],
        )
        if agents:
            options.agents = agents
        if mcp_servers:
            options.mcp_servers = mcp_servers
        if disallowed_mcp_tools:
            options.disallowed_tools = disallowed_mcp_tools
        if sdk_session_id:
            # Resume the previous SDK session. The platform looked up
            # this id from the prior run's attrSDKSessionID Record
            # attribute; the SDK reads its own JSONL transcript from
            # <cwd>/.claude/projects/<id>.jsonl (which is symlinked to
            # NS_SESSION_STATE_DIR via _link_session_state_dir).
            options.resume = sdk_session_id
            logger.info("resuming SDK session %s", sdk_session_id)

        # Attachment metadata lands in the system prompt rather than the
        # user message so resumed sessions don't replay it as new turns.
        # Bytes are fetched on demand via the download_artifact MCP tool —
        # see artifact_tools.py.
        attachments = await _describe_session_attachments(api)
        if attachments:
            lines = "\n".join(
                f"- id={a['id']}  name={a['name']}  size={a['size_bytes']}  type={a['content_type']}"
                for a in attachments
            )
            options.system_prompt = (
                "The user attached files to this conversation. Each is "
                "identified by an artifact id below. Call "
                "`download_artifact(artifact_id)` to materialize one onto "
                "disk; the tool returns the absolute path, which you can "
                "then pass to Read / Grep / Bash. Only download files you "
                "actually need to answer the user.\n\n"
                + lines
            )

        logger.info("starting agent for run %s", RUN_ID)
        event_index = 0

        try:
            async with ClaudeSDKClient(options=options) as client:
                await client.query(RUN_PROMPT)

                async for message in client.receive_messages():
                    # Cancellation poll.
                    if (
                        event_index > 0
                        and event_index % CANCEL_POLL_INTERVAL == 0
                    ):
                        if await api.poll_cancellation():
                            logger.info(
                                "run %s cancelled, interrupting", RUN_ID
                            )
                            await client.interrupt()
                            async for msg in client.receive_response():
                                if isinstance(msg, ResultMessage):
                                    break
                            await api.complete("", None)
                            return

                    # Serialize and POST event.
                    raw = serialize_message(message)
                    await api.emit(message_type(message), raw)
                    event_index += 1

                    # Terminal message.
                    if isinstance(message, ResultMessage):
                        sdk_id = getattr(message, "session_id", None) or ""

                        if message.is_error:
                            await api.fail(
                                message.result or "unknown error"
                            )
                        else:
                            usage_attr = (
                                getattr(message, "usage", None) or {}
                            )
                            usage_payload = {
                                "input_tokens": int(
                                    usage_attr.get("input_tokens", 0) or 0
                                ),
                                "output_tokens": int(
                                    usage_attr.get("output_tokens", 0) or 0
                                ),
                                "cache_read_tokens": int(
                                    usage_attr.get(
                                        "cache_read_input_tokens", 0
                                    )
                                    or 0
                                ),
                                "cache_creation_tokens": int(
                                    usage_attr.get(
                                        "cache_creation_input_tokens", 0
                                    )
                                    or 0
                                ),
                                "total_cost_usd": float(
                                    getattr(message, "total_cost_usd", 0)
                                    or 0
                                ),
                            }
                            # chunk-14 round-trip: push the just-written
                            # SDK transcript back to the API before
                            # marking the run COMPLETED. Sequencing
                            # matters: DeleteSession's active-run guard
                            # refuses delete while RUNNING, so the
                            # upload-vs-delete race is closed.
                            if ss_client is not None:
                                try:
                                    uploaded, failed = await ss_client.upload_from(
                                        Path(SESSION_STATE_DIR)
                                    )
                                    logger.info(
                                        "session-state: uploaded=%d failed=%d",
                                        uploaded,
                                        failed,
                                    )
                                except Exception:
                                    logger.exception(
                                        "session-state: upload failed; continuing"
                                    )
                            await api.complete(sdk_id, usage_payload)
                        return

            # Exited the async-for without a ResultMessage — treat as
            # clean exit with empty session id.
            await api.complete("", None)

        except Exception as e:
            logger.exception("run %s failed", RUN_ID)
            try:
                await api.fail(str(e))
            except httpx.HTTPError:
                logger.error("failed to report error to API")
            sys.exit(1)


if __name__ == "__main__":
    import asyncio

    asyncio.run(run())
