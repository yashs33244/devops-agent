import { cronProxy } from "@/lib/server/cron-proxy";

type Ctx = { params: Promise<{ id: string }> };

export async function POST(request: Request, { params }: Ctx) {
  const { id } = await params;
  const body = await request.json();
  return cronProxy(`/artifacts/${encodeURIComponent(id)}/share`, {
    method: "POST",
    body,
  });
}
