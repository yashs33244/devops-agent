import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";
import { genericOAuth } from "better-auth/plugins/generic-oauth";
import { eq } from "drizzle-orm";
import { getDb } from "@/db";
import { user as userTable } from "@/db/schema/user";

const oidcEnabled =
  !!process.env.OIDC_CLIENT_ID && !!process.env.OIDC_CLIENT_SECRET;

function roleFromGroups(groups: unknown): "admin" | "user" {
  return Array.isArray(groups) && groups.includes("admin") ? "admin" : "user";
}

function decodeIdToken(idToken: string | undefined): Record<string, unknown> | null {
  if (!idToken) return null;
  const parts = idToken.split(".");
  if (parts.length < 2) return null;
  try {
    return JSON.parse(Buffer.from(parts[1]!, "base64url").toString("utf8"));
  } catch {
    return null;
  }
}

async function syncRoleFromAccount(account: unknown): Promise<void> {
  const acc = account as { idToken?: string | null; userId?: string };
  const payload = decodeIdToken(acc.idToken ?? undefined);
  if (!payload || !acc.userId) return;
  const role = roleFromGroups(payload.groups);
  await getDb()
    .update(userTable)
    .set({ role, updatedAt: new Date() })
    .where(eq(userTable.id, acc.userId));
}

export const auth = betterAuth({
  database: drizzleAdapter(getDb(), { provider: "pg" }),
  emailAndPassword: {
    // OIDC is the only sanctioned login path. Set AUTH_EMAIL_PASSWORD_ENABLED=true
    // temporarily to re-enable the legacy fallback if the OIDC hop is down.
    enabled: process.env.AUTH_EMAIL_PASSWORD_ENABLED === "true",
  },
  user: {
    additionalFields: {
      role: {
        type: "string",
        defaultValue: "user",
      },
    },
  },
  plugins: oidcEnabled
    ? [
        genericOAuth({
          config: [
            {
              providerId: "openbao",
              authorizationUrl: process.env.OIDC_AUTHORIZATION_URL!,
              tokenUrl: process.env.OIDC_TOKEN_URL!,
              userInfoUrl: process.env.OIDC_USERINFO_URL!,
              clientId: process.env.OIDC_CLIENT_ID!,
              clientSecret: process.env.OIDC_CLIENT_SECRET!,
              scopes: ["openid", "groups"],
              pkce: true,
              // OpenBao's userinfo isn't reachable through the UI ingress, so
              // pull identity straight out of the id_token.
              getUserInfo: async (tokens) => {
                const payload = decodeIdToken(
                  (tokens as { idToken?: string }).idToken,
                );
                if (!payload) return null;
                const sub = String(payload.sub ?? "");
                if (!sub) return null;
                return {
                  id: sub,
                  name: (payload.name as string) || sub,
                  email: (payload.email as string) || `${sub}@openbao.local`,
                  emailVerified: true,
                  createdAt: new Date(),
                  updatedAt: new Date(),
                };
              },
            },
          ],
        }),
      ]
    : [],
  databaseHooks: {
    // Sync user.role from the id_token's groups claim on every OIDC login
    // (both first-time account creation AND subsequent re-logins). Uses
    // Drizzle directly to avoid depending on internal Better-Auth APIs.
    account: {
      create: {
        after: async (account) => {
          await syncRoleFromAccount(account);
        },
      },
      update: {
        after: async (account) => {
          await syncRoleFromAccount(account);
        },
      },
    },
  },
  secret: process.env.BETTER_AUTH_SECRET,
  baseURL: process.env.BETTER_AUTH_URL,
  trustedOrigins: (request) => {
    const explicit = (process.env.BETTER_AUTH_TRUSTED_ORIGINS || "")
      .split(",")
      .filter(Boolean);
    const origin = request?.headers?.get?.("origin");
    if (origin) {
      try {
        const host = new URL(origin).hostname;
        if (host.endsWith(".vercel.app")) return [...explicit, origin];
      } catch {}
    }
    return explicit;
  },
});

export type Session = typeof auth.$Infer.Session.session;
export type User = typeof auth.$Infer.Session.user;
