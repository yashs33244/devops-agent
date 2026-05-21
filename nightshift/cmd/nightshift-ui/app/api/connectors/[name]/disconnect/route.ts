import { cronProxy } from "@/lib/server/cron-proxy";

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ name: string }> },
) {
  const { name } = await params;
  return cronProxy(`/connectors/${encodeURIComponent(name)}/disconnect`, {
    method: "POST",
  });
}
