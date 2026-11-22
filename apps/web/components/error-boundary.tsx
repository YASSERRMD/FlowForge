"use client";

import * as React from "react";
import { AlertTriangle } from "lucide-react";

import { Button } from "@/components/ui/button";

export function ErrorFallback({
  title = "Something went wrong",
  description = "An unexpected error occurred while rendering this section.",
  error,
  reset,
}: {
  title?: string;
  description?: string;
  error?: Error;
  reset?: () => void;
}) {
  return (
    <div
      role="alert"
      className="flex flex-col items-center gap-3 rounded-lg border border-error-muted bg-error-muted/40 p-8 text-center"
    >
      <AlertTriangle className="h-8 w-8 text-error" aria-hidden="true" />
      <div>
        <h2 className="text-lg font-semibold">{title}</h2>
        <p className="mt-1 text-sm text-muted-foreground">{description}</p>
        {error?.message ? (
          <p className="mt-2 rounded bg-muted px-2 py-1 font-mono text-xs text-muted-foreground">
            {error.message}
          </p>
        ) : null}
      </div>
      {reset ? (
        <Button variant="outline" onClick={reset}>
          Try again
        </Button>
      ) : null}
    </div>
  );
}

interface ErrorBoundaryProps {
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

export class ErrorBoundary extends React.Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error("ErrorBoundary caught:", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        this.props.fallback ?? (
          <ErrorFallback
            error={this.state.error}
            reset={() => this.setState({ error: null })}
          />
        )
      );
    }
    return this.props.children;
  }
}
