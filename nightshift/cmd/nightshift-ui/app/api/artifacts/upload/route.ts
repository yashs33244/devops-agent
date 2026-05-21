import { cronProxy } from "@/lib/server/cron-proxy";

// Anything larger should use a presigned-PUT flow instead of inline
// base64 bytes.
const MAX_BODY_BYTES = 40 * 1024 * 1024;

export async function POST(request: Request) {
  const len = Number(request.headers.get("content-length") || 0);
  if (len > MAX_BODY_BYTES) {
    return Response.json({ error: "File too large" }, { status: 413 });
  }
  const body = await request.json();
  return cronProxy("/artifacts/upload", { method: "POST", body });
}
