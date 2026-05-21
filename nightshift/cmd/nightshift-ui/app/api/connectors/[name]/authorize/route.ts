import { getSession } from "@/lib/server/auth";
import { env } from "@/lib/server/env";
import { getOidcIdToken, getOidcSubject } from "@/lib/server/oidc-token";

// OAuth init: POST :startOAuthFlow, then 302 the browser to the
// provider's authorize_url. The provider eventually redirects back to
// the sibling oauth-callback route.
export async function GET(
  _request: Request,
  { params }: { params: Promise<{ name: string }> },
): Promise<Response> {
  const session = await getSession();
  if (!session) return Response.json({ error: "Unauthorized" }, { status: 401 });

  const { name } = await params;
  const sub = await getOidcSubject(session.user.id);
  const idToken = await getOidcIdToken(session.user.id);
  if (!sub || !idToken) {
    return new Response("OIDC session expired — sign in again", { status: 401 });
  }

  // Must match a redirect URI registered on the provider's OAuth app.
  const redirectUrl = `${env.BETTER_AUTH_URL}/api/connectors/${encodeURIComponent(name)}/oauth-callback`;

  const upstream = await fetch(
    `${env.NIGHTSHIFT_API_URL}/v1/connectors/${encodeURIComponent(name)}:startOAuthFlow`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${idToken}`,
      },
      body: JSON.stringify({ user_id: sub, connector_name: name, redirect_url: redirectUrl }),
      signal: AbortSignal.timeout(15_000),
    },
  );

  if (!upstream.ok) {
    const body = await upstream.text();
    return new Response(body || "OAuth init failed", {
      status: upstream.status || 502,
      headers: { "Content-Type": "text/plain; charset=utf-8" },
    });
  }

  const data = (await upstream.json()) as { authorizeUrl?: string; authorize_url?: string };
  const authorizeUrl = data.authorizeUrl ?? data.authorize_url;
  if (!authorizeUrl) {
    return new Response("OAuth init returned no authorize_url", { status: 502 });
  }
  return Response.redirect(authorizeUrl, 302);
}
