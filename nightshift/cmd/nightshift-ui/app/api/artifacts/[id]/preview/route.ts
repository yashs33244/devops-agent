import { cronBinaryProxy } from "@/lib/server/cron-proxy";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  return cronBinaryProxy(`/artifacts/${id}/preview`);
}
