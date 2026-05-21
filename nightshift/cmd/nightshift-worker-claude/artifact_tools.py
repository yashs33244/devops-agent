"""SDK MCP tools for agent artifact deployment.

Provides custom tools that agents can call during execution to create,
list, update, and share artifacts. Tool handlers POST to internal API
routes — the agent never interacts with S3/K8s directly.
"""

from __future__ import annotations

import base64
import logging
import os
from pathlib import Path
from typing import Any

import httpx
from claude_agent_sdk import create_sdk_mcp_server, tool

logger = logging.getLogger("nightshift-worker-claude.artifact_tools")

TOOL_SERVER_NAME = "nightshift"


def create_artifact_tools(
    api_base_url: str,
    run_id: str,
    user_id: str,
    headers: dict[str, str] | None = None,
    session_id: str = "",
):
    """Build an SDK MCP server with artifact tools.

    When session_id is set, it is stamped on agent-produced artifacts so
    /v1/artifacts?session_id=X returns both user uploads and agent outputs.

    Returns (server_config, allowed_tools_list).
    """
    _headers = dict(headers) if headers else {}
    _base = api_base_url.rstrip("/")

    # ── download_artifact ──

    @tool(
        "download_artifact",
        "Fetch a user-uploaded session attachment onto disk and return "
        "its absolute path. Use the artifact_id from the attachment list "
        "in the system prompt. Returns the path; read it with the Read "
        "tool. If the file is already on disk from a prior turn, returns "
        "the cached path without re-fetching.",
        {
            "type": "object",
            "properties": {
                "artifact_id": {
                    "type": "string",
                    "description": "Artifact id (e.g. art_…) from the attachment list",
                },
            },
            "required": ["artifact_id"],
        },
    )
    async def download_artifact(args: dict[str, Any]) -> dict[str, Any]:
        artifact_id = (args.get("artifact_id") or "").strip()
        if not artifact_id:
            return {
                "content": [{"type": "text", "text": "artifact_id is required"}],
                "is_error": True,
            }
        workspace = os.getenv("NS_WORKSPACE", "/home/nightshift/workspace")
        uploads_dir = Path(workspace) / "uploads"
        uploads_dir.mkdir(parents=True, exist_ok=True)

        async with httpx.AsyncClient(timeout=60.0, headers=_headers) as http:
            # Look up metadata + presigned URL in one shot.
            info_resp = await http.get(f"{_base}/v1/artifacts/{artifact_id}")
            if info_resp.status_code != 200:
                return {
                    "content": [
                        {
                            "type": "text",
                            "text": f"Artifact lookup failed: {info_resp.status_code} {info_resp.text[:200]}",
                        }
                    ],
                    "is_error": True,
                }
            art = info_resp.json().get("artifact", {})
            name = art.get("name") or ""
            size_bytes = int(art.get("sizeBytes") or art.get("size_bytes") or 0)
            safe = Path(name).name
            if not safe or safe.startswith("."):
                return {
                    "content": [{"type": "text", "text": f"Unsafe artifact name: {name!r}"}],
                    "is_error": True,
                }
            # Prefix with the artifact id stub so two uploads named the
            # same don't clobber each other across turns.
            disk_name = f"{artifact_id[:8]}_{safe}" if artifact_id.startswith("art_") else safe
            target = uploads_dir / disk_name

            if target.exists() and target.stat().st_size == size_bytes:
                logger.info("download_artifact: cache hit %s (%d bytes)", target, size_bytes)
                return {
                    "content": [{"type": "text", "text": str(target)}],
                }

            url_resp = await http.get(f"{_base}/v1/artifacts/{artifact_id}:downloadUrl")
            if url_resp.status_code != 200:
                return {
                    "content": [
                        {
                            "type": "text",
                            "text": f"Download URL failed: {url_resp.status_code} {url_resp.text[:200]}",
                        }
                    ],
                    "is_error": True,
                }
            data = url_resp.json()
            url = data.get("downloadUrl") or data.get("download_url") or ""
            if not url:
                return {
                    "content": [{"type": "text", "text": "No download URL returned"}],
                    "is_error": True,
                }
        # Bare client (no worker auth headers) — the presigned URL's
        # SigV4 signature is computed against an empty header set, so
        # extra headers cause MinIO/S3 to return 400.
        try:
            async with httpx.AsyncClient(timeout=60.0) as blob_http:
                blob = await blob_http.get(url)
        except httpx.HTTPError as e:
            return {
                "content": [{"type": "text", "text": f"Fetch failed: {str(e)[:200]}"}],
                "is_error": True,
            }
        if blob.status_code != 200:
            return {
                "content": [
                    {"type": "text", "text": f"Fetch failed: HTTP {blob.status_code}"}
                ],
                "is_error": True,
            }
        target.write_bytes(blob.content)
        logger.info(
            "download_artifact: wrote %s (%d bytes, reported size=%d)",
            target,
            len(blob.content),
            size_bytes,
        )
        return {"content": [{"type": "text", "text": str(target)}]}

    # ── deploy_app ──

    @tool(
        "deploy_app",
        "Deploy an HTML application as a hosted static site. "
        "Provide the complete, self-contained HTML content (with inline CSS/JS). "
        "Returns the artifact ID and access URL.",
        {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Short display name for the app",
                },
                "description": {
                    "type": "string",
                    "description": "Brief description of what the app does",
                },
                "content": {
                    "type": "string",
                    "description": "Complete HTML content to deploy (inline all CSS/JS)",
                },
                "public": {
                    "type": "boolean",
                    "description": "Whether the app is publicly accessible (default false)",
                },
            },
            "required": ["name", "content"],
        },
    )
    async def deploy_app(args: dict[str, Any]) -> dict[str, Any]:
        async with httpx.AsyncClient(timeout=60.0, headers=_headers) as http:
            resp = await http.post(
                f"{_base}/v1/internal/artifacts/deploy-app",
                json={
                    "name": args["name"],
                    "description": args.get("description", ""),
                    "html_content": args["content"],   # content → html_content
                    "public": args.get("public", False),
                    "owner_id": user_id,                # user_id → owner_id
                    "run_id": run_id,
                    "session_id": session_id,
                },
            )
            if resp.status_code not in (200, 201):
                return {
                    "content": [{"type": "text", "text": f"Deploy failed: {resp.text}"}],
                    "is_error": True,
                }
            info = resp.json().get("artifact", {})
            return {
                "content": [
                    {
                        "type": "text",
                        "text": (
                            f"App deployed successfully.\n"
                            f"Artifact ID: {info.get('id', '')}\n"
                            f"Name: {info.get('name', '')}\n"
                            f"URL: {info.get('appUrl', info.get('app_url', ''))}\n"
                            f"Public: {info.get('public', False)}"
                        ),
                    }
                ]
            }

    # ── deploy_object ──

    @tool(
        "deploy_object",
        "Upload a file or data object to persistent storage. "
        "Content must be base64-encoded. Returns the artifact ID and filename.",
        {
            "type": "object",
            "properties": {
                "filename": {
                    "type": "string",
                    "description": "Filename with extension (e.g. report.csv, analysis.json)",
                },
                "content_base64": {
                    "type": "string",
                    "description": "Base64-encoded file content",
                },
                "content_type": {
                    "type": "string",
                    "description": "MIME type (e.g. text/csv, application/pdf, application/json)",
                },
            },
            "required": ["filename", "content_base64", "content_type"],
        },
    )
    async def deploy_object(args: dict[str, Any]) -> dict[str, Any]:
        # Validate base64 early
        try:
            base64.b64decode(args["content_base64"])
        except Exception:
            return {
                "content": [{"type": "text", "text": "Error: invalid base64 content"}],
                "is_error": True,
            }

        async with httpx.AsyncClient(timeout=60.0, headers=_headers) as http:
            resp = await http.post(
                f"{_base}/v1/internal/artifacts/deploy-object",
                json={
                    "name": args["filename"],
                    "content": args["content_base64"],
                    "content_type": args.get("content_type", "application/octet-stream"),
                    "owner_id": user_id,
                    "run_id": run_id,
                    "session_id": session_id,
                },
            )
            if resp.status_code not in (200, 201):
                return {
                    "content": [{"type": "text", "text": f"Upload failed: {resp.text}"}],
                    "is_error": True,
                }
            info = resp.json().get("artifact", {})
            return {
                "content": [
                    {
                        "type": "text",
                        "text": (
                            f"Object uploaded successfully.\n"
                            f"Artifact ID: {info.get('id', '')}\n"
                            f"Filename: {info.get('name', '')}\n"
                            f"Size: {info.get('sizeBytes', info.get('size_bytes', 0))} bytes\n"
                            f"Content-Type: {info.get('contentType', info.get('content_type', ''))}"
                        ),
                    }
                ]
            }

    # ── list_artifacts ──

    @tool(
        "list_artifacts",
        "List artifacts in the current chat thread. Pass scope='all' "
        "to widen to every artifact the user owns. Optionally filter "
        "by type.",
        {
            "type": "object",
            "properties": {
                "type": {
                    "type": "string",
                    "enum": ["app", "object"],
                    "description": "Filter by artifact type (omit for all)",
                },
                "scope": {
                    "type": "string",
                    "enum": ["session", "all"],
                    "description": "session (default) or all",
                },
            },
            "required": [],
        },
    )
    async def list_artifacts(args: dict[str, Any]) -> dict[str, Any]:
        # Proto field names: owner_id (not user_id), type_filter as the
        # full enum string (ARTIFACT_TYPE_OBJECT/APP).
        params: dict[str, str] = {"owner_id": user_id}
        scope = args.get("scope", "session")
        if scope == "session" and session_id:
            params["session_id"] = session_id
        artifact_type = args.get("type", "")
        if artifact_type:
            params["type_filter"] = (
                "ARTIFACT_TYPE_APP" if artifact_type == "app" else "ARTIFACT_TYPE_OBJECT"
            )

        async with httpx.AsyncClient(timeout=30.0, headers=_headers) as http:
            resp = await http.get(
                f"{_base}/v1/internal/artifacts/list",
                params=params,
            )
            if resp.status_code != 200:
                return {
                    "content": [{"type": "text", "text": f"List failed: {resp.text}"}],
                    "is_error": True,
                }
            payload = resp.json()
            artifacts = payload.get("artifacts", [])
            if not artifacts:
                return {
                    "content": [{"type": "text", "text": "No artifacts found."}]
                }

            lines = []
            for a in artifacts:
                url = a.get("appUrl", a.get("app_url", "")) or f"/artifacts/{a['id']}/download"
                kind = a.get("type", "")
                if kind == "ARTIFACT_TYPE_OBJECT":
                    kind = "object"
                elif kind == "ARTIFACT_TYPE_APP":
                    kind = "app"
                lines.append(
                    f"- [{kind}] {a['name']} (ID: {a['id']})\n"
                    f"  URL: {url} | Public: {a.get('public', False)}"
                )
            return {
                "content": [
                    {
                        "type": "text",
                        "text": f"Found {len(artifacts)} artifact(s):\n\n"
                        + "\n".join(lines),
                    }
                ]
            }

    # ── update_artifact ──

    @tool(
        "update_artifact",
        "Update an existing artifact's content, metadata, or visibility. "
        "For apps, provide new HTML content. For objects, provide new base64 content. "
        "Can also change name, description, or toggle public/private.",
        {
            "type": "object",
            "properties": {
                "artifact_id": {
                    "type": "string",
                    "description": "ID of the artifact to update",
                },
                "content": {
                    "type": "string",
                    "description": "New HTML content (app artifacts only)",
                },
                "content_base64": {
                    "type": "string",
                    "description": "New base64-encoded content (object artifacts only)",
                },
                "name": {
                    "type": "string",
                    "description": "New display name",
                },
                "description": {
                    "type": "string",
                    "description": "New description",
                },
                "public": {
                    "type": "boolean",
                    "description": "Set to true for public access, false for private",
                },
            },
            "required": ["artifact_id"],
        },
    )
    async def update_artifact(args: dict[str, Any]) -> dict[str, Any]:
        # Proto fields: name/description/public for metadata, content_bytes
        # (bytes; grpc-gateway accepts base64 string) + content_type for
        # object blob replacement, html_content for app rewrites.
        body: dict[str, Any] = {}
        if "name" in args:
            body["name"] = args["name"]
        if "description" in args:
            body["description"] = args["description"]
        if "public" in args:
            body["public"] = args["public"]
        if "content_base64" in args:
            body["content_bytes"] = args["content_base64"]
            # content_type defaults preserved server-side when omitted.
            if "content_type" in args:
                body["content_type"] = args["content_type"]
        if "content" in args:
            body["html_content"] = args["content"]

        async with httpx.AsyncClient(timeout=60.0, headers=_headers) as http:
            resp = await http.put(
                f"{_base}/v1/internal/artifacts/{args['artifact_id']}/update",
                json=body,
            )
            if resp.status_code != 200:
                return {
                    "content": [{"type": "text", "text": f"Update failed: {resp.text}"}],
                    "is_error": True,
                }
            info = resp.json().get("artifact", {})
            url = info.get("appUrl", info.get("app_url", "")) or f"/artifacts/{info.get('id','')}/download"
            return {
                "content": [
                    {
                        "type": "text",
                        "text": (
                            f"Artifact updated successfully.\n"
                            f"Artifact ID: {info.get('id', '')}\n"
                            f"Name: {info.get('name', '')}\n"
                            f"URL: {url}\n"
                            f"Public: {info.get('public', False)}"
                        ),
                    }
                ]
            }

    # ── share_artifact ──

    @tool(
        "share_artifact",
        "Grant or revoke access to an artifact for another user.",
        {
            "type": "object",
            "properties": {
                "artifact_id": {
                    "type": "string",
                    "description": "ID of the artifact to share",
                },
                "target_user_id": {
                    "type": "string",
                    "description": "User ID to grant/revoke access for",
                },
                "role": {
                    "type": "string",
                    "enum": ["viewer", "editor"],
                    "description": "Permission role to grant (default: viewer)",
                },
                "revoke": {
                    "type": "boolean",
                    "description": "Set to true to revoke access instead of granting",
                },
            },
            "required": ["artifact_id", "target_user_id"],
        },
    )
    async def share_artifact(args: dict[str, Any]) -> dict[str, Any]:
        # Proto split: ShareArtifact (POST) for grants, RevokeArtifactShare
        # (DELETE /permissions/{user_id}) for removals. cr0n's
        # revoke=true flag is mapped to the DELETE method here.
        artifact_id = args["artifact_id"]
        target = args["target_user_id"]
        role_str = args.get("role", "viewer")
        proto_role = (
            "ARTIFACT_ROLE_EDITOR" if role_str == "editor" else "ARTIFACT_ROLE_VIEWER"
        )
        async with httpx.AsyncClient(timeout=30.0, headers=_headers) as http:
            if args.get("revoke"):
                resp = await http.delete(
                    f"{_base}/v1/artifacts/{artifact_id}/permissions/{target}",
                )
                action = "revoked"
            else:
                resp = await http.post(
                    f"{_base}/v1/internal/artifacts/{artifact_id}/share",
                    json={"user_id": target, "role": proto_role},
                )
                action = "granted"
            if resp.status_code != 200:
                return {
                    "content": [{"type": "text", "text": f"Share failed: {resp.text}"}],
                    "is_error": True,
                }
            return {
                "content": [
                    {
                        "type": "text",
                        "text": (
                            f"Permission {action} for user {target} "
                            f"on artifact {artifact_id}."
                        ),
                    }
                ]
            }

    # ── show_preview_artifact ──

    @tool(
        "show_preview_artifact",
        "Display an inline live preview of an artifact in the chat UI. "
        "Works for both app artifacts (HTML apps deployed via deploy_app) and "
        "object artifacts (files uploaded via deploy_object — images, PDFs, "
        "markdown, JSON, CSV, text). Call this immediately after deploying or "
        "uploading so the user sees a visual preview inline. Clicking the "
        "preview opens the full-size artifact viewer panel. No-op on the "
        "backend — the UI reads everything from the tool_use event input.",
        {
            "type": "object",
            "properties": {
                "artifact_id": {
                    "type": "string",
                    "description": "ID returned by deploy_app or deploy_object",
                },
                "name": {
                    "type": "string",
                    "description": "Display name of the artifact",
                },
                "type": {
                    "type": "string",
                    "enum": ["app", "object"],
                    "description": (
                        "Artifact kind. Default 'app'. Use 'object' for files "
                        "uploaded via deploy_object."
                    ),
                },
                "url": {
                    "type": "string",
                    "description": (
                        "Access URL — required for type='app' (pass the "
                        "app_url from deploy_app). Omit for type='object'; "
                        "the UI builds its own view URL."
                    ),
                },
                "content_type": {
                    "type": "string",
                    "description": (
                        "MIME type — required for type='object' (e.g. "
                        "'image/png', 'application/pdf', 'text/markdown', "
                        "'application/json', 'text/csv', 'text/plain'). Pass "
                        "the exact content_type returned by deploy_object."
                    ),
                },
                "caption": {
                    "type": "string",
                    "description": "Optional short caption rendered above the preview",
                },
            },
            "required": ["artifact_id", "name"],
        },
    )
    async def show_preview_artifact(args: dict[str, Any]) -> dict[str, Any]:
        # Intentional no-op. The UI reads artifact_id/name/type/url/
        # content_type/caption directly from the tool_use.input block in the
        # event stream; we only return a short text ack so the agent gets a
        # conversational confirmation.
        kind = args.get("type", "app")
        if kind == "object":
            label = f"{args.get('content_type', 'object')} object"
        else:
            label = "app"
        return {
            "content": [
                {
                    "type": "text",
                    "text": (
                        f"Preview displayed inline for {args['name']} "
                        f"({args['artifact_id']}, {label})."
                    ),
                }
            ]
        }

    # ── Report generation tools (PDF / DOCX / XLSX) ──

    DOCX_MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    XLSX_MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
    PPTX_MIME = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

    async def _deploy_report(
        *,
        filename: str,
        binary: bytes,
        content_type: str,
        preview_html: str,
    ) -> dict[str, Any]:
        """Shared helper: POST a generated report + its HTML preview to the
        internal deploy-object route. Returns the ArtifactInfo JSON on
        success, or an MCP error dict the tool can return directly."""
        body = {
            "name": filename,
            "content": base64.b64encode(binary).decode("ascii"),
            "content_type": content_type,
            "owner_id": user_id,
            "run_id": run_id,
            "session_id": session_id,
            "preview_html": base64.b64encode(
                preview_html.encode("utf-8")
            ).decode("ascii"),
        }
        async with httpx.AsyncClient(timeout=60.0, headers=_headers) as http:
            resp = await http.post(
                f"{_base}/v1/internal/artifacts/deploy-object", json=body
            )
            if resp.status_code not in (200, 201):
                return {
                    "content": [
                        {
                            "type": "text",
                            "text": f"Upload failed: {resp.text}",
                        }
                    ],
                    "is_error": True,
                }
            return {"info": resp.json().get("artifact", resp.json())}

    def _success_text(info: dict[str, Any], content_type: str) -> dict[str, Any]:
        return {
            "content": [
                {
                    "type": "text",
                    "text": (
                        f"Report created successfully.\n"
                        f"Artifact ID: {info.get('id', '')}\n"
                        f"Filename: {info.get('name', '')}\n"
                        f"Size: {info.get('sizeBytes', info.get('size_bytes', 0))} bytes\n"
                        f"Content-Type: {content_type}"
                    ),
                }
            ]
        }

    # ── create_pdf ──

    @tool(
        "create_pdf",
        "Create a PDF report from Markdown content. Use this for one-page "
        "summaries, multi-page reports, or any deliverable the user wants "
        "in PDF. Content is rendered with print-friendly styling (letter "
        "size, 0.75in margins). The PDF is uploaded as an object artifact "
        "and registered with the run. Follow up with show_preview_artifact "
        "(type='object', content_type='application/pdf').",
        {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Filename (should end in .pdf)",
                },
                "content": {
                    "type": "string",
                    "description": (
                        "Report content in Markdown — supports headings, "
                        "lists, tables, code blocks, images, blockquotes"
                    ),
                },
                "description": {
                    "type": "string",
                    "description": "Short description",
                },
            },
            "required": ["name", "content"],
        },
    )
    async def create_pdf(args: dict[str, Any]) -> dict[str, Any]:
        try:
            from report_generators import render_pdf

            pdf_bytes, preview_html = render_pdf(
                title=args["name"],
                markdown_content=args["content"],
            )
        except Exception as e:  # noqa: BLE001
            logger.exception("create_pdf failed")
            return {
                "content": [
                    {"type": "text", "text": f"PDF generation failed: {e}"}
                ],
                "is_error": True,
            }

        filename = (
            args["name"]
            if args["name"].lower().endswith(".pdf")
            else f"{args['name']}.pdf"
        )
        result = await _deploy_report(
            filename=filename,
            binary=pdf_bytes,
            content_type="application/pdf",
            preview_html=preview_html,
        )
        if "info" not in result:
            return result
        return _success_text(result["info"], "application/pdf")

    # ── create_docx ──

    @tool(
        "create_docx",
        "Create a Microsoft Word (.docx) document from Markdown content. "
        "Use this when the user wants an editable report. The docx is "
        "uploaded as an object artifact and registered with the run. "
        "Follow up with show_preview_artifact (type='object', "
        "content_type='application/vnd.openxmlformats-officedocument.wordprocessingml.document').",
        {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Filename (should end in .docx)",
                },
                "content": {
                    "type": "string",
                    "description": (
                        "Report content in Markdown — supports headings, "
                        "lists, tables, and basic formatting"
                    ),
                },
                "description": {
                    "type": "string",
                    "description": "Short description",
                },
            },
            "required": ["name", "content"],
        },
    )
    async def create_docx(args: dict[str, Any]) -> dict[str, Any]:
        try:
            from report_generators import render_docx

            docx_bytes, preview_html = render_docx(
                title=args["name"],
                markdown_content=args["content"],
            )
        except Exception as e:  # noqa: BLE001
            logger.exception("create_docx failed")
            return {
                "content": [
                    {"type": "text", "text": f"DOCX generation failed: {e}"}
                ],
                "is_error": True,
            }

        filename = (
            args["name"]
            if args["name"].lower().endswith(".docx")
            else f"{args['name']}.docx"
        )
        result = await _deploy_report(
            filename=filename,
            binary=docx_bytes,
            content_type=DOCX_MIME,
            preview_html=preview_html,
        )
        if "info" not in result:
            return result
        return _success_text(result["info"], DOCX_MIME)

    # ── create_xlsx ──

    @tool(
        "create_xlsx",
        "Create a Microsoft Excel (.xlsx) workbook from a structured sheet "
        "spec. Use this when the user wants tabular data they can pivot, "
        "filter, or chart. Supports multiple sheets. Each sheet has a "
        "name, a header row, and data rows. Numbers should be passed as "
        "numeric values (not strings) so Excel treats them as numbers. "
        "Follow up with show_preview_artifact (type='object', "
        "content_type='application/vnd.openxmlformats-officedocument.spreadsheetml.sheet').",
        {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Filename (should end in .xlsx)",
                },
                "description": {
                    "type": "string",
                    "description": "Short description",
                },
                "sheets": {
                    "type": "array",
                    "description": "One or more worksheets",
                    "items": {
                        "type": "object",
                        "properties": {
                            "name": {
                                "type": "string",
                                "description": "Sheet tab name (max 31 chars)",
                            },
                            "headers": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": "Header row (column names)",
                            },
                            "rows": {
                                "type": "array",
                                "items": {"type": "array"},
                                "description": (
                                    "Data rows; each row is an array of "
                                    "cell values (strings or numbers)"
                                ),
                            },
                        },
                        "required": ["name", "headers", "rows"],
                    },
                },
            },
            "required": ["name", "sheets"],
        },
    )
    async def create_xlsx(args: dict[str, Any]) -> dict[str, Any]:
        try:
            from report_generators import render_xlsx

            xlsx_bytes, preview_html = render_xlsx(
                title=args["name"],
                sheets=args["sheets"],
            )
        except Exception as e:  # noqa: BLE001
            logger.exception("create_xlsx failed")
            return {
                "content": [
                    {"type": "text", "text": f"XLSX generation failed: {e}"}
                ],
                "is_error": True,
            }

        filename = (
            args["name"]
            if args["name"].lower().endswith(".xlsx")
            else f"{args['name']}.xlsx"
        )
        result = await _deploy_report(
            filename=filename,
            binary=xlsx_bytes,
            content_type=XLSX_MIME,
            preview_html=preview_html,
        )
        if "info" not in result:
            return result
        return _success_text(result["info"], XLSX_MIME)

    # ── create_pptx ──

    @tool(
        "create_pptx",
        "Create a Microsoft PowerPoint (.pptx) deck from a structured slide "
        "spec. Use this when the user wants slides — an executive pitch, a "
        "project kickoff, a readout, a training deck, etc. Each slide picks "
        "from a small set of layouts: 'title' (cover slide with title + "
        "optional subtitle), 'section' (section divider), 'bullets' (title "
        "+ bullet list — the most common), 'content' (title + paragraph of "
        "body text), 'two_column' (title + two bullet columns). Follow up "
        "with show_preview_artifact (type='object', "
        "content_type='application/vnd.openxmlformats-officedocument.presentationml.presentation').",
        {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Filename (should end in .pptx)",
                },
                "description": {
                    "type": "string",
                    "description": "Short description",
                },
                "slides": {
                    "type": "array",
                    "description": (
                        "Ordered list of slides. Each slide has a 'layout' "
                        "field plus layout-specific fields."
                    ),
                    "items": {
                        "type": "object",
                        "properties": {
                            "layout": {
                                "type": "string",
                                "enum": [
                                    "title",
                                    "section",
                                    "bullets",
                                    "content",
                                    "two_column",
                                ],
                                "description": "Slide layout",
                            },
                            "title": {
                                "type": "string",
                                "description": "Slide title / heading",
                            },
                            "subtitle": {
                                "type": "string",
                                "description": "Used by 'title' and 'section' layouts",
                            },
                            "bullets": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": (
                                    "Bullet items — used by 'bullets' layout"
                                ),
                            },
                            "body": {
                                "type": "string",
                                "description": "Paragraph body — used by 'content' layout",
                            },
                            "left": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": (
                                    "Left column bullets — 'two_column' layout"
                                ),
                            },
                            "right": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": (
                                    "Right column bullets — 'two_column' layout"
                                ),
                            },
                        },
                        "required": ["layout", "title"],
                    },
                },
            },
            "required": ["name", "slides"],
        },
    )
    async def create_pptx(args: dict[str, Any]) -> dict[str, Any]:
        try:
            from report_generators import render_pptx

            pptx_bytes, preview_html = render_pptx(
                title=args["name"],
                slides=args["slides"],
            )
        except Exception as e:  # noqa: BLE001
            logger.exception("create_pptx failed")
            return {
                "content": [
                    {"type": "text", "text": f"PPTX generation failed: {e}"}
                ],
                "is_error": True,
            }

        filename = (
            args["name"]
            if args["name"].lower().endswith(".pptx")
            else f"{args['name']}.pptx"
        )
        result = await _deploy_report(
            filename=filename,
            binary=pptx_bytes,
            content_type=PPTX_MIME,
            preview_html=preview_html,
        )
        if "info" not in result:
            return result
        return _success_text(result["info"], PPTX_MIME)

    # ── create_schedule ──

    @tool(
        "create_schedule",
        "Schedule a prompt to run automatically on a cron cadence. Use this "
        "when the user asks you to set up a recurring job — a daily report, "
        "a weekly digest, a periodic check. The prompt will be fired as a "
        "fresh run by a K8s CronJob using the user's default agent "
        "configuration. Use standard 5-field cron syntax (e.g. '0 9 * * *' "
        "for 9am daily). Returns the schedule id.",
        {
            "type": "object",
            "properties": {
                "prompt": {
                    "type": "string",
                    "description": "The prompt the schedule will fire with each tick",
                },
                "cron": {
                    "type": "string",
                    "description": (
                        "5-field cron expression, e.g. '*/15 * * * *' "
                        "(every 15 min), '0 9 * * 1' (Mondays 9am)"
                    ),
                },
                "timezone": {
                    "type": "string",
                    "description": "IANA timezone name. Defaults to UTC.",
                },
                "enabled": {
                    "type": "boolean",
                    "description": "Start active. Defaults to true.",
                },
            },
            "required": ["prompt", "cron"],
        },
    )
    async def create_schedule(args: dict[str, Any]) -> dict[str, Any]:
        # Proto canonical fields: prompt, cron, timezone, enabled,
        # session_id, user_id (no run_id — schedules persist across runs).
        body = {
            "prompt": args["prompt"],
            "cron": args["cron"],
            "timezone": args.get("timezone", "UTC"),
            "enabled": args.get("enabled", True),
            "user_id": user_id,
        }

        async with httpx.AsyncClient(timeout=30.0, headers=_headers) as http:
            resp = await http.post(
                f"{_base}/v1/internal/schedules", json=body,
            )
            if resp.status_code not in (200, 201):
                return {
                    "content": [
                        {"type": "text", "text": f"Schedule create failed: {resp.text}"}
                    ],
                    "is_error": True,
                }
            info = resp.json().get("schedule", {})
            return {
                "content": [
                    {
                        "type": "text",
                        "text": (
                            f"Schedule created.\n"
                            f"Schedule ID: {info.get('id', '')}\n"
                            f"Prompt: {info.get('prompt', '')}\n"
                            f"Cron: {info.get('cron', '')} ({info.get('timezone', 'UTC')})\n"
                            f"Enabled: {info.get('enabled', True)}"
                        ),
                    }
                ]
            }

    # Build the MCP server
    server = create_sdk_mcp_server(
        name=TOOL_SERVER_NAME,
        version="1.0.0",
        tools=[
            download_artifact,
            deploy_app,
            deploy_object,
            list_artifacts,
            update_artifact,
            share_artifact,
            show_preview_artifact,
            create_pdf,
            create_docx,
            create_xlsx,
            create_pptx,
            create_schedule,
        ],
    )

    allowed = [
        f"mcp__{TOOL_SERVER_NAME}__download_artifact",
        f"mcp__{TOOL_SERVER_NAME}__deploy_app",
        f"mcp__{TOOL_SERVER_NAME}__deploy_object",
        f"mcp__{TOOL_SERVER_NAME}__list_artifacts",
        f"mcp__{TOOL_SERVER_NAME}__update_artifact",
        f"mcp__{TOOL_SERVER_NAME}__share_artifact",
        f"mcp__{TOOL_SERVER_NAME}__show_preview_artifact",
        f"mcp__{TOOL_SERVER_NAME}__create_pdf",
        f"mcp__{TOOL_SERVER_NAME}__create_docx",
        f"mcp__{TOOL_SERVER_NAME}__create_xlsx",
        f"mcp__{TOOL_SERVER_NAME}__create_pptx",
        f"mcp__{TOOL_SERVER_NAME}__create_schedule",
    ]

    return server, allowed
