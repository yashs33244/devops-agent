import { cronProxy } from "@/lib/server/cron-proxy";

type Ctx = { params: Promise<{ id: string }> };

export async function PUT(request: Request, { params }: Ctx) {
  const { id } = await params;
  const body = await request.json();
  return cronProxy(`/artifacts/${encodeURIComponent(id)}/edit`, {
    method: "PUT",
    body,
  });
}

export async function DELETE(_request: Request, { params }: Ctx) {
  const { id } = await params;
  return cronProxy(`/artifacts/${encodeURIComponent(id)}/delete`, {
    method: "DELETE",
  });
}
