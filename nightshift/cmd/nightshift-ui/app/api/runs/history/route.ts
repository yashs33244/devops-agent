import { NextRequest } from "next/server";
import { cronProxy } from "@/lib/server/cron-proxy";

export async function GET(request: NextRequest) {
  const sessionId = request.nextUrl.searchParams.get("session_id") || "";
  return cronProxy("/runs/history/list", { query: { session_id: sessionId } });
}
