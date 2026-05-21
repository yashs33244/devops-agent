import { cronSSEProxy } from "@/lib/server/cron-proxy";

// SSE: must run uncached on the Node runtime, otherwise the response
// body gets buffered until the upstream stream closes.
export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const fetchCache = "force-no-store";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ runId: string }> },
) {
  const { runId } = await params;
  return cronSSEProxy(`/runs/${runId}/events/sse`);
}
