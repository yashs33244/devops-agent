import { cronProxy } from "@/lib/server/cron-proxy";

export async function GET() {
  return cronProxy("/schedule/list");
}

export async function POST(request: Request) {
  const body = await request.json();
  return cronProxy("/schedule/create", { method: "POST", body });
}
