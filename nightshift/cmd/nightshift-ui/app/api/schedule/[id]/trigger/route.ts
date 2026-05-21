import { cronProxy } from "@/lib/server/cron-proxy";

type Ctx = { params: Promise<{ id: string }> };

export async function POST(_request: Request, { params }: Ctx) {
  const { id } = await params;
  return cronProxy(`/schedule/${encodeURIComponent(id)}/trigger`, { method: "POST" });
}
