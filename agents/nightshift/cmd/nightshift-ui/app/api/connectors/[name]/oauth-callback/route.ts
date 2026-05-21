import { getSession } from "@/lib/server/auth";
import { env } from "@/lib/server/env";
import { getOidcIdToken, getOidcSubject } from "@/lib/server/oidc-token";

// OAuth callback: provider redirects here with ?code&state. We hand
// them to :completeOAuthFlow, then bounce to /connectors with a flag
// the UI uses to flash a success/error chip.
export async function GET(
  request: Request,
  { params }: { params: Promise<{ name: string }> },
): Promise<Response> {
  const session = await getSession();
  if (!session) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const { name } = await params;
  const url = new URL(request.url);
  const providerError = url.searchParams.get("error");
  if (providerError) {
    const desc = url.searchParams.get("error_description") || providerError;
    return Response.redirect(
      `${env.BETTER_AUTH_URL}/connectors?error=${encodeURIComponent(desc)}`,
      302,
    );
  }

  const code = url.searchParams.get("code") ?? "";
  const state = url.searchParams.get("state") ?? "";
  if (!code || !state) return new Response("Missing code or state", { status: 400 });

  const sub = await getOidcSubject(session.user.id);
  const idToken = await getOidcIdToken(session.user.id);
  if (!sub || !idToken) {
    return new Response("OIDC session expired — sign in again", { status: 401 });
  }

  // redirect_url at exchange must match the one used at authorize.
  const redirectUrl = `${env.BETTER_AUTH_URL}/api/connectors/${encodeURIComponent(name)}/oauth-callback`;

  const upstream = await fetch(
    `${env.NIGHTSHIFT_API_URL}/v1/connectors/${encodeURIComponent(name)}:completeOAuthFlow`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${idToken}`,
      },
      body: JSON.stringify({
        user_id: sub,
        connector_name: name,
        code,
        state,
        redirect_url: redirectUrl,
      }),
      signal: AbortSignal.timeout(15_000),
    },
  );

  if (!upstream.ok) {
    const body = await upstream.text();
    return Response.redirect(
      `${env.BETTER_AUTH_URL}/connectors?error=${encodeURIComponent(body || `HTTP ${upstream.status}`)}`,
      302,
    );
  }

  return Response.redirect(
    `${env.BETTER_AUTH_URL}/connectors?connected=${encodeURIComponent(name)}`,
    302,
  );
}
