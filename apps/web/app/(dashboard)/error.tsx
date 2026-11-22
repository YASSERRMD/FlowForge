"use client";

import { useEffect } from "react";
import { toast } from "sonner";

import { ErrorFallback } from "@/components/error-boundary";

export default function DashboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    toast.error("Dashboard section failed to load", {
      description: error.message,
    });
  }, [error]);

  return (
    <ErrorFallback
      title="This section hit an error"
      description="The rest of the dashboard is unaffected. You can retry safely."
      error={error}
      reset={reset}
    />
  );
}
