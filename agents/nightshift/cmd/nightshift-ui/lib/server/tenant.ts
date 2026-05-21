import { getSession } from "@/lib/server/auth";
import { env } from "@/lib/server/env";

export type TenantAuth = {
  session: {
    user: {
      id: string;
      name: string;
      email: string;
      role?: string;
    };
  };
  org: {
    id: string;
    name: string;
  };
};

function extractAuth(session: NonNullable<Awaited<ReturnType<typeof getSession>>>): TenantAuth {
  return {
    session: {
      user: {
        id: session.user.id,
        name: session.user.name,
        email: session.user.email,
        role: (session.user as Record<string, unknown>).role as string | undefined,
      },
    },
    org: {
      id: env.TENANT_ID,
      name: env.TENANT_NAME,
    },
  };
}

export async function requireTenantAuth(): Promise<TenantAuth | null> {
  const session = await getSession();
  if (!session?.user) return null;
  return extractAuth(session);
}

export async function requireAdminAuth(): Promise<TenantAuth | Response> {
  const session = await getSession();
  if (!session?.user) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }
  const auth = extractAuth(session);
  if (auth.session.user.role !== "admin") {
    return Response.json({ error: "Forbidden" }, { status: 403 });
  }
  return auth;
}
