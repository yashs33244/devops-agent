"""OpenBao client for OAuth tokens (oauthapp plugin) and KV v2 secrets."""

from __future__ import annotations

import logging
import time
from pathlib import Path

import httpx

logger = logging.getLogger("nightshift-worker-claude.bao")

_SA_TOKEN_PATH = "/var/run/secrets/kubernetes.io/serviceaccount/token"


class BaoClient:
    """Manages OAuth tokens and KV secrets through OpenBao."""

    def __init__(self, bao_url: str, bao_token: str = "") -> None:
        self._bao_url = bao_url.rstrip("/")
        self._bao_token = bao_token
        self._http: httpx.AsyncClient | None = None
        self._token_expires: float = 0.0
        self._k8s_role: str = ""

    @property
    def _headers(self) -> dict[str, str]:
        return {"X-Vault-Token": self._bao_token}

    async def start(self) -> None:
        self._http = httpx.AsyncClient(timeout=15.0)

    async def close(self) -> None:
        if self._http:
            await self._http.aclose()

    # -- Kubernetes auth -------------------------------------------------------

    async def login_kubernetes(self, role: str, jwt: str | None = None) -> bool:
        """Authenticate using a Kubernetes ServiceAccount token."""
        assert self._http is not None
        if jwt is None:
            try:
                jwt = Path(_SA_TOKEN_PATH).read_text().strip()
            except FileNotFoundError:
                logger.error("SA token not found at %s — not running in K8s?", _SA_TOKEN_PATH)
                return False
        self._k8s_role = role
        resp = await self._http.post(
            f"{self._bao_url}/v1/auth/kubernetes/login",
            json={"role": role, "jwt": jwt},
        )
        if resp.status_code == 200:
            auth = resp.json().get("auth", {})
            self._bao_token = auth["client_token"]
            ttl = auth.get("lease_duration", 3600)
            self._token_expires = time.time() + ttl
            logger.info("authenticated to openbao as role=%s (ttl=%ds)", role, ttl)
            return True
        logger.error("openbao k8s login failed: %d %s", resp.status_code, resp.text)
        return False

    async def _ensure_token(self) -> None:
        """Re-login if the current token is near expiry."""
        if self._k8s_role and self._token_expires and time.time() > self._token_expires - 60:
            logger.info("openbao token near expiry, re-authenticating")
            await self.login_kubernetes(self._k8s_role)

    # -- KV v2 -----------------------------------------------------------------

    async def kv_read(self, path: str) -> dict | None:
        """Read a KV v2 secret. *path* is relative, e.g. ``cr0n/anthropic-api-key``."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.get(
            f"{self._bao_url}/v1/secret/data/{path}",
            headers=self._headers,
        )
        if resp.status_code == 200:
            return resp.json().get("data", {}).get("data", {})
        logger.warning("kv_read %s failed: %d", path, resp.status_code)
        return None

    async def kv_write(self, path: str, data: dict) -> bool:
        """Write a KV v2 secret."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.post(
            f"{self._bao_url}/v1/secret/data/{path}",
            headers=self._headers,
            json={"data": data},
        )
        if resp.status_code in (200, 204):
            return True
        logger.warning("kv_write %s failed: %d", path, resp.status_code)
        return False

    async def kv_delete(self, path: str) -> bool:
        """Permanently delete a KV v2 secret (metadata + all versions)."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.delete(
            f"{self._bao_url}/v1/secret/metadata/{path}",
            headers=self._headers,
        )
        if resp.status_code in (200, 204, 404):
            return True
        logger.warning("kv_delete %s failed: %d", path, resp.status_code)
        return False

    # -- OAuth (oauthapp plugin) ------------------------------------------------

    async def register_server(
        self,
        name: str,
        provider: str,
        client_id: str,
        client_secret: str,
        provider_options: dict | None = None,
        auth_url_params: dict | None = None,
    ) -> bool:
        """Register an OAuth provider server in OpenBao.

        For known providers (github, google, etc.), no extra options needed.
        For 'custom' provider, pass provider_options with auth_code_url + token_url.
        For 'oidc' provider, pass provider_options with issuer_url.

        auth_url_params are extra query params appended to the authorization
        URL — e.g. Dropbox requires ``token_access_type=offline`` to issue a
        refresh_token.
        """
        assert self._http is not None
        await self._ensure_token()
        body: dict = {
            "provider": provider,
            "client_id": client_id,
            "client_secret": client_secret,
        }
        if provider_options:
            body["provider_options"] = provider_options
        if auth_url_params:
            body["auth_url_params"] = auth_url_params
        resp = await self._http.put(
            f"{self._bao_url}/v1/oauth2/servers/{name}",
            headers=self._headers,
            json=body,
        )
        if resp.status_code in (200, 204):
            logger.info("registered bao server: %s (provider=%s)", name, provider)
            return True
        logger.warning("failed to register bao server %s: %d %s", name, resp.status_code, resp.text)
        return False

    async def get_auth_url(
        self,
        server: str,
        scopes: list[str],
        state: str,
        redirect_url: str,
    ) -> str | None:
        """Generate an OAuth authorization URL for a user to visit."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.put(
            f"{self._bao_url}/v1/oauth2/auth-code-url",
            headers=self._headers,
            json={
                "server": server,
                "scopes": scopes,
                "state": state,
                "redirect_url": redirect_url,
            },
        )
        if resp.status_code == 200:
            data = resp.json().get("data", {})
            return data.get("url")
        logger.warning("failed to get auth URL for %s: %d", server, resp.status_code)
        return None

    async def exchange_code(
        self,
        cred_name: str,
        server: str,
        code: str,
        redirect_url: str,
    ) -> bool:
        """Exchange an authorization code for tokens and store in OpenBao."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.put(
            f"{self._bao_url}/v1/oauth2/creds/{cred_name}",
            headers=self._headers,
            json={
                "server": server,
                "code": code,
                "redirect_url": redirect_url,
                "grant_type": "authorization_code",
            },
        )
        if resp.status_code in (200, 204):
            logger.info("stored cred %s", cred_name)
            return True
        logger.warning("failed to exchange code for %s: %d %s", cred_name, resp.status_code, resp.text)
        return False

    async def get_token(self, cred_name: str) -> str | None:
        """Read a fresh access token. OpenBao auto-refreshes if expired."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.get(
            f"{self._bao_url}/v1/oauth2/creds/{cred_name}",
            headers=self._headers,
        )
        if resp.status_code == 200:
            data = resp.json().get("data", {})
            return data.get("access_token")
        if resp.status_code == 404:
            return None  # No credential stored for this user/connector
        logger.warning("failed to read cred %s: %d", cred_name, resp.status_code)
        return None

    async def delete_cred(self, cred_name: str) -> bool:
        """Delete a stored credential."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.delete(
            f"{self._bao_url}/v1/oauth2/creds/{cred_name}",
            headers=self._headers,
        )
        if resp.status_code in (200, 204):
            logger.info("deleted cred %s", cred_name)
            return True
        return False

    # -- Identity (user directory) ---------------------------------------------

    async def list_entity_names(self) -> list[str]:
        """List all identity entity names. Returns e.g. ["gianni", "alice"]."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.request(
            "LIST",
            f"{self._bao_url}/v1/identity/entity/name",
            headers=self._headers,
        )
        if resp.status_code == 200:
            return resp.json().get("data", {}).get("keys", []) or []
        if resp.status_code == 404:
            return []
        logger.warning("list_entity_names failed: %d", resp.status_code)
        return []

    async def get_entity_by_name(self, name: str) -> dict | None:
        """Return the entity's {id, name, ...metadata} or None if missing."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.get(
            f"{self._bao_url}/v1/identity/entity/name/{name}",
            headers=self._headers,
        )
        if resp.status_code == 200:
            return resp.json().get("data")
        if resp.status_code == 404:
            return None
        logger.warning("get_entity_by_name %s failed: %d", name, resp.status_code)
        return None

    async def has_cred(self, cred_name: str) -> bool:
        """Check if a credential exists."""
        assert self._http is not None
        await self._ensure_token()
        resp = await self._http.get(
            f"{self._bao_url}/v1/oauth2/creds/{cred_name}",
            headers=self._headers,
        )
        return resp.status_code == 200
