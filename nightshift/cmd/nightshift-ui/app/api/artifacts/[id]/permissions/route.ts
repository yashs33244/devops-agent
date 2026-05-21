import { cronProxy } from "@/lib/server/cron-proxy";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(_request: Request, { params }: Ctx) {
  const { id } = await params;
  return cronProxy(`/artifacts/${encodeURIComponent(id)}/permissions`);
}
