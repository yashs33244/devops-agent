import { cronProxy } from "@/lib/server/cron-proxy";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ name: string }> },
) {
  const { name } = await params;
  const body = await request.json();
  return cronProxy(`/connectors/${encodeURIComponent(name)}/token`, {
    method: "POST",
    body,
  });
}
