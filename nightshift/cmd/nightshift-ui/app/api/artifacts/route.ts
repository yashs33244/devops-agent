import { cronProxy } from "@/lib/server/cron-proxy";

export async function GET() {
  return cronProxy("/artifacts/list");
}
