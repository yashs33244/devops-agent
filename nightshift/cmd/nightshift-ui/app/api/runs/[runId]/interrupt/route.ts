import { cronProxy } from "@/lib/server/cron-proxy";

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ runId: string }> },
) {
  const { runId } = await params;
  return cronProxy(`/runs/${runId}/interrupt`, { method: "POST", body: {} });
}
