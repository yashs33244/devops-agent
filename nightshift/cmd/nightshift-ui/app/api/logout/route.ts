import { and, eq } from "drizzle-orm";
import { headers } from "next/headers";
import { auth } from "@/auth";
import { getDb } from "@/db";
import { account as accountTable } from "@/db/schema/auth";
import { getSession } from "@/lib/server/auth";
import { env } from "@/lib/server/env";

/**
 * Log out.
 *
 * Two things have to happen for logout to stick:
 *  1. Clear the better-auth session cookie — `auth.api.signOut` does this.
 *  2. Scrub the stored OpenBao id_token on the user's account row so the
 *     silent-reauth path in cron-proxy.ts can't resurrect access using a
 *     still-valid JWT after the user clicked Log out.
 *
 * No browser-side OpenBao cookie to clear: the userpass-bridge at
 * /login/openbao authenticates against OpenBao server-side only, so the
 * user's bao token never reaches the browser.
 */
export async function POST() {
  const h = await headers();

  // Best-effort id_token scrub — run before signOut so the account row
  // survives long enough for us to find it by session.user.id.
  try {
    const session = await getSession();
    if (session?.user?.id) {
      await getDb()
        .update(accountTable)
        .set({
          idToken: null,
          accessToken: null,
          refreshToken: null,
          accessTokenExpiresAt: null,
          refreshTokenExpiresAt: null,
        })
        .where(
          and(
            eq(accountTable.userId, session.user.id),
            eq(accountTable.providerId, "openbao"),
          ),
        );
    }
  } catch {
    // non-fatal
  }

  try {
    await auth.api.signOut({ headers: h });
  } catch {
    // non-fatal — cookie may already be gone
  }

  return Response.redirect(new URL("/login", env.BETTER_AUTH_URL).toString(), 302);
}
