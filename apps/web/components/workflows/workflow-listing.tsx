"use client";

import * as React from "react";
import { LayoutGrid, List, Search } from "lucide-react";

import { cn } from "@/lib/utils";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { WorkflowStatusChip } from "@/components/workflows/workflow-status-chip";
import {
  DEMO_WORKFLOWS,
  type DemoWorkflow,
  type WorkflowStatus,
} from "@/lib/demo/workflows";

type ViewMode = "cards" | "list";
type StatusFilter = "all" | WorkflowStatus;

function OwnerBadge({ owner }: { owner: string }) {
  const initials = owner
    .split(" ")
    .map((word) => word[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
  return (
    <span className="flex items-center gap-2 text-sm text-muted-foreground">
      <span
        aria-hidden="true"
        className="flex h-6 w-6 items-center justify-center rounded-full bg-secondary text-[10px] font-semibold text-secondary-foreground"
      >
        {initials}
      </span>
      {owner}
    </span>
  );
}

function WorkflowCard({ workflow }: { workflow: DemoWorkflow }) {
  return (
    <article className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 text-card-foreground">
      <div className="flex items-start justify-between gap-2">
        <h3 className="font-semibold leading-tight">{workflow.name}</h3>
        <WorkflowStatusChip status={workflow.status} />
      </div>
      <p className="text-sm text-muted-foreground">{workflow.description}</p>
      <div className="mt-auto flex items-center justify-between pt-2">
        <OwnerBadge owner={workflow.owner} />
        <span className="text-xs text-muted-foreground">
          {workflow.runCount.toLocaleString()} runs
        </span>
      </div>
    </article>
  );
}

export function WorkflowListing() {
  const [query, setQuery] = React.useState("");
  const [status, setStatus] = React.useState<StatusFilter>("all");
  const [view, setView] = React.useState<ViewMode>("cards");

  const filtered = DEMO_WORKFLOWS.filter((workflow) => {
    const matchesStatus = status === "all" || workflow.status === status;
    const q = query.trim().toLowerCase();
    const matchesQuery =
      !q ||
      workflow.name.toLowerCase().includes(q) ||
      workflow.description.toLowerCase().includes(q) ||
      workflow.owner.toLowerCase().includes(q);
    return matchesStatus && matchesQuery;
  });

  return (
    <section aria-label="Workflow list" className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative max-w-xs flex-1">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search workflows…"
            aria-label="Search workflows"
            className="pl-9"
          />
        </div>
        <div className="flex items-center gap-2">
          <Tabs
            value={status}
            onValueChange={(value) => setStatus(value as StatusFilter)}
          >
            <TabsList aria-label="Filter by status">
              <TabsTrigger value="all">All</TabsTrigger>
              <TabsTrigger value="active">Active</TabsTrigger>
              <TabsTrigger value="draft">Draft</TabsTrigger>
              <TabsTrigger value="paused">Paused</TabsTrigger>
              <TabsTrigger value="error">Error</TabsTrigger>
            </TabsList>
          </Tabs>
          <div
            role="group"
            aria-label="View mode"
            className="flex rounded-md border border-border"
          >
            <Button
              variant={view === "cards" ? "secondary" : "ghost"}
              size="icon"
              aria-label="Card view"
              aria-pressed={view === "cards"}
              onClick={() => setView("cards")}
            >
              <LayoutGrid className="h-4 w-4" aria-hidden="true" />
            </Button>
            <Button
              variant={view === "list" ? "secondary" : "ghost"}
              size="icon"
              aria-label="List view"
              aria-pressed={view === "list"}
              onClick={() => setView("list")}
            >
              <List className="h-4 w-4" aria-hidden="true" />
            </Button>
          </div>
        </div>
      </div>

      <p
        className="text-xs text-muted-foreground"
        role="note"
        aria-label="Demo data notice"
      >
        Demo data — workflows are static placeholders until the API ships.
      </p>

      {filtered.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border p-10 text-center">
          <p className="font-medium">No workflows found</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Try a different search or status filter.
          </p>
        </div>
      ) : view === "cards" ? (
        <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {filtered.map((workflow) => (
            <li key={workflow.id}>
              <WorkflowCard workflow={workflow} />
            </li>
          ))}
        </ul>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {filtered.map((workflow) => (
            <li
              key={workflow.id}
              className={cn(
                "flex flex-col gap-2 p-4 sm:flex-row sm:items-center sm:justify-between",
              )}
            >
              <div>
                <div className="flex items-center gap-2">
                  <h3 className="font-medium">{workflow.name}</h3>
                  <WorkflowStatusChip status={workflow.status} />
                </div>
                <p className="mt-1 text-sm text-muted-foreground">
                  {workflow.description}
                </p>
              </div>
              <div className="flex items-center gap-4">
                <OwnerBadge owner={workflow.owner} />
                <Badge variant="outline">
                  {workflow.runCount.toLocaleString()} runs
                </Badge>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
