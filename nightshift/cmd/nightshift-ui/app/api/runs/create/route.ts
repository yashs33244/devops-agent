import { cronProxy } from "@/lib/server/cron-proxy";

export async function POST(request: Request) {
  const body = await request.json();
  return cronProxy("/runs/create", { method: "POST", body });
}
