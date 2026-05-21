import { cronBinaryProxy } from "@/lib/server/cron-proxy";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(_request: Request, { params }: Ctx) {
  const { id } = await params;
  return cronBinaryProxy(`/artifacts/${encodeURIComponent(id)}/download`);
}
