import { cronProxy } from "@/lib/server/cron-proxy";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ runId: string }> },
) {
  const { runId } = await params;
  // Long history fetches on multi-minute runs can blow past the 30s
  // default — bump to 2 minutes. This is the SSE-recovery fallback
  // path (see useChat's `finalizeFromHistory`); a slow response here
  // should always wait, never abort and finalize as error.
  return cronProxy(`/runs/${runId}/events/history`, { timeout: 120_000 });
}
