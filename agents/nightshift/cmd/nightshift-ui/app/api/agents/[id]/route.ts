import { cronProxy } from "@/lib/server/cron-proxy";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(_request: Request, { params }: Ctx) {
  const { id } = await params;
  return cronProxy(`/agents/${encodeURIComponent(id)}/info`);
}

export async function PUT(request: Request, { params }: Ctx) {
  const { id } = await params;
  const body = await request.json();
  return cronProxy(`/agents/${encodeURIComponent(id)}/edit`, {
    method: "PUT",
    body,
  });
}

export async function DELETE(_request: Request, { params }: Ctx) {
  const { id } = await params;
  return cronProxy(`/agents/${encodeURIComponent(id)}/delete`, {
    method: "DELETE",
  });
}
