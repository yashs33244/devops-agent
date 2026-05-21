import { and, eq } from "drizzle-orm";
import { getDb } from "@/db";
import { account as accountTable } from "@/db/schema/auth";

const EXPIRY_SKEW_MS = 60_000;

/**
 * Fetch the current OpenBao id_token for a user. Returns null when the
 * stored token is missing or already expired.
 *
 * OpenBao 2.2 does not issue refresh tokens in authorization code flow, so
 * there's nothing to refresh server-side. When this returns null the caller
 * should let the request 401 back to the browser; the UI handles a 401 by
 * triggering a silent `prompt=none` re-authorization against OpenBao's
 * browser session cookie.
 */
export async function getOidcIdToken(userId: string): Promise<string | null> {
  const rows = await getDb()
    .select({
      idToken: accountTable.idToken,
      accessTokenExpiresAt: accountTable.accessTokenExpiresAt,
    })
    .from(accountTable)
    .where(
      and(
        eq(accountTable.userId, userId),
        eq(accountTable.providerId, "openbao"),
      ),
    )
    .limit(1);

  const row = rows[0];
  if (!row || !row.idToken) return null;
  const exp = row.accessTokenExpiresAt?.getTime();
  if (exp && exp - Date.now() < EXPIRY_SKEW_MS) return null;
  return row.idToken;
}

/**
 * Read the OIDC `sub` claim out of the user's stored id_token. This is
 * the canonical user identity nightshift-api derives from the bearer
 * token. Better-auth's `session.user.id` is its own locally-generated
 * primary key and does NOT match `sub`, so any per-user query that
 * needs to round-trip through the API (artifacts, schedules, etc.)
 * MUST use this value, not session.user.id, otherwise the API's
 * cross-tenant collapse rule returns empty results.
 */
export async function getOidcSubject(userId: string): Promise<string | null> {
  const idToken = await getOidcIdToken(userId);
  if (!idToken) return null;
  const parts = idToken.split(".");
  if (parts.length < 2) return null;
  try {
    const payload = JSON.parse(
      Buffer.from(parts[1]!, "base64url").toString("utf8"),
    ) as { sub?: unknown };
    return typeof payload.sub === "string" ? payload.sub : null;
  } catch {
    return null;
  }
}
