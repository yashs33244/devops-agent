import { cronProxy } from "@/lib/server/cron-proxy";

export async function GET() {
  return cronProxy("/skills/list");
}

export async function POST(request: Request) {
  const body = await request.json();
  return cronProxy("/skills/create", { method: "POST", body });
}
