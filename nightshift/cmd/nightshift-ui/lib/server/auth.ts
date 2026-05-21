import { auth } from "@/auth";
import { headers } from "next/headers";

export async function getSession() {
  const h = await headers();
  const session = await auth.api.getSession({ headers: h });
  return session;
}
