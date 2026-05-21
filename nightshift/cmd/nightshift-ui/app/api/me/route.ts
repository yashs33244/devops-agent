import { and, eq } from "drizzle-orm";
import { getDb } from "@/db";
import { account as accountTable } from "@/db/schema/auth";
import { user as userTable } from "@/db/schema/user";
import { getSession } from "@/lib/server/auth";

/**
 * Current-user identity for the UI, keyed by the OIDC `sub` claim that
 * cr0n-a uses for every user_id column. `session.user.id` is Better-Auth's
 * own opaque primary key — it does NOT match owner_id / invoker_id /
 * artifact_permissions.user_id, so it's useless for comparing against
 * cr0n-a data. Clients should hit this endpoint instead of reading
 * session.user.id directly when they need to say "this row belongs to me."
 */
export async function GET() {
  const session = await getSession();
  if (!session?.user) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const rows = await getDb()
    .select({
      accountId: accountTable.accountId,
      name: userTable.name,
      email: userTable.email,
      role: userTable.role,
    })
    .from(accountTable)
    .innerJoin(userTable, eq(userTable.id, accountTable.userId))
    .where(
      and(
        eq(accountTable.userId, session.user.id),
        eq(accountTable.providerId, "openbao"),
      ),
    )
    .limit(1);

  const row = rows[0];
  if (!row) {
    // Email+password session — no linked OpenBao account, so no OIDC sub.
    // Fall back to Better-Auth's user.id; downstream features that need a
    // real sub will no-op rather than mismatch.
    return Response.json({
      id: session.user.id,
      name: session.user.name,
      email: session.user.email,
      role: (session.user as { role?: string }).role ?? "user",
    });
  }

  return Response.json({
    id: row.accountId,
    name: row.name,
    email: row.email,
    role: row.role,
  });
}
