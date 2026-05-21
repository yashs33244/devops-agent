"use client";

import { useEffect } from "react";

export default function DashboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("[dashboard] unhandled error:", error);
  }, [error]);

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4 px-4">
      <div className="size-12 flex items-center justify-center rounded-xl border border-error/30 bg-error/10 text-error text-xl font-bold">
        !
      </div>
      <h2 className="text-lg font-semibold text-primary">Something went wrong</h2>
      <p className="text-sm text-muted max-w-md text-center">
        {error.message || "An unexpected error occurred."}
      </p>
      <button
        onClick={reset}
        className="mt-2 rounded-lg bg-lime text-night px-4 py-2 text-sm font-semibold hover:bg-lime-bright focus-ring"
      >
        Try again
      </button>
    </div>
  );
}
