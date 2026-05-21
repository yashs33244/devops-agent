import { cronProxy } from "@/lib/server/cron-proxy";

type Ctx = { params: Promise<{ id: string }> };

export async function DELETE(_request: Request, { params }: Ctx) {
  const { id } = await params;
  return cronProxy(`/runs/session/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}
