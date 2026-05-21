import { env } from "@/lib/server/env";

const FORM_FIELDS = new Set(["username", "password"]);

/**
 * Bridges the missing OpenBao web UI: drives userpass auth + OIDC
 * authorize server-side, then 302s the browser to better-auth's
 * existing OAuth callback. The form on /login/openbao POSTs here.
 *
 * Implemented as a Route Handler (not a Server Action) so the
 * browser sees a real HTTP redirect chain — Server Action redirects
 * are turned into RSC soft-navigations, which can't follow the API
 * route's 302 → /tasks → set-cookie → final-page chain reliably.
 */
export async function POST(request: Request): Promise<Response> {
  const formData = await request.formData();

  const username = (formData.get("username") || "").toString().trim();
  const password = (formData.get("password") || "").toString();

  // Capture the OAuth params verbatim. better-auth-issued `state` and
  // `code_challenge` must round-trip unchanged or its later token
  // exchange will reject the PKCE verifier.
  const oauthParams = new URLSearchParams();
  let redirectUri = "";
  for (const [key, value] of formData.entries()) {
    if (FORM_FIELDS.has(key)) continue;
    const v = value.toString();
    oauthParams.set(key, v);
    if (key === "redirect_uri") redirectUri = v;
  }

  // Helper: bounce back to the form with all params + an inline error.
  const back = (errorMsg: string): Response => {
    const params = new URLSearchParams(oauthParams);
    params.set("error", errorMsg);
    const url = new URL(
      `/login/openbao?${params.toString()}`,
      env.BETTER_AUTH_URL,
    );
    return Response.redirect(url.toString(), 303);
  };

  if (!username || !password) {
    return back("Username and password are required.");
  }
  if (!redirectUri || !oauthParams.get("state")) {
    return back("Missing OAuth parameters — restart sign-in from /login.");
  }

  const base = env.OPENBAO_INTERNAL_URL.replace(/\/$/, "");

  let token: string | null = null;
  try {
    const res = await fetch(
      `${base}/v1/auth/userpass/login/${encodeURIComponent(username)}`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
        cache: "no-store",
      },
    );
    if (res.status === 400 || res.status === 401 || res.status === 403) {
      return back("Invalid username or password.");
    }
    if (!res.ok) {
      return back(`OpenBao userpass login failed (${res.status}).`);
    }
    const body = (await res.json()) as { auth?: { client_token?: string } };
    token = body.auth?.client_token ?? null;
  } catch (err) {
    console.error("[openbao-bridge] userpass login fetch error", err);
    return back("Could not reach OpenBao. Try again.");
  }
  if (!token) return back("OpenBao did not return a client token.");

  // Authorize with X-Vault-Token returns top-level {code, state} JSON.
  // Errors come back as {error, error_description, state}.
  type AuthorizeBody = {
    code?: string;
    state?: string;
    error?: string;
    error_description?: string;
  };
  let authorizeBody: AuthorizeBody | null = null;
  let authorizeStatus = 0;
  try {
    const res = await fetch(
      `${base}/v1/identity/oidc/provider/nightshift/authorize?${oauthParams.toString()}`,
      {
        method: "GET",
        headers: { "X-Vault-Token": token, Accept: "application/json" },
        cache: "no-store",
      },
    );
    authorizeStatus = res.status;
    authorizeBody = (await res.json().catch(() => null)) as AuthorizeBody | null;
  } catch (err) {
    console.error("[openbao-bridge] authorize fetch error", err);
    return back("Could not complete OpenBao authorize. Try again.");
  }

  if (
    authorizeStatus < 200 ||
    authorizeStatus >= 300 ||
    authorizeBody?.error ||
    !authorizeBody?.code
  ) {
    const detail =
      authorizeBody?.error_description ||
      authorizeBody?.error ||
      `authorize failed (${authorizeStatus})`;
    return back(`Sign-in denied: ${detail}`);
  }

  const target = new URL(redirectUri);
  target.searchParams.set("state", authorizeBody.state ?? "");
  target.searchParams.set("code", authorizeBody.code);
  return Response.redirect(target.toString(), 303);
}
