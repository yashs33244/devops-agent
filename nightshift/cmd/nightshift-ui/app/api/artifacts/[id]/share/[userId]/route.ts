import { cronProxy } from "@/lib/server/cron-proxy";

type Ctx = { params: Promise<{ id: string; userId: string }> };

export async function DELETE(_request: Request, { params }: Ctx) {
  const { id, userId } = await params;
  return cronProxy(
    `/artifacts/${encodeURIComponent(id)}/share/${encodeURIComponent(userId)}`,
    { method: "DELETE" },
  );
}
