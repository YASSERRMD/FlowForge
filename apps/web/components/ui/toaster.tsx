"use client";

import { useTheme } from "next-themes";
import { Toaster as SonnerToaster } from "sonner";

export function Toaster() {
  const { resolvedTheme } = useTheme();

  return (
    <SonnerToaster
      theme={resolvedTheme === "dark" ? "dark" : "light"}
      position="bottom-right"
      richColors
      closeButton
      toastOptions={{
        classNames: {
          toast: "border border-border bg-card text-card-foreground",
        },
      }}
    />
  );
}
