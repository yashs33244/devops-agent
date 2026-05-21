import { cronProxy } from "@/lib/server/cron-proxy";

export async function GET() {
  return cronProxy("/agents/list");
}

export async function POST(request: Request) {
  const body = await request.json();
  return cronProxy("/agents/create", { method: "POST", body });
}
