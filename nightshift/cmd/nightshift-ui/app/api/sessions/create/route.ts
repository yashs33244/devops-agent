import { cronProxy } from "@/lib/server/cron-proxy";

export async function POST() {
  return cronProxy("/sessions/create", { method: "POST", body: {} });
}
