"use client";

import * as React from "react";

import {
  createBrowserClient,
  type VersionDetail,
  type VersionSummary,
} from "@/lib/api/client";
import type { WorkflowGraph } from "@/lib/contracts/graph";
import { diffGraphs, summarizeDiff, type GraphDiff } from "@/lib/editor/diff";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

function statusVariant(status: VersionSummary["status"]) {
  return status === "published" ? "default" : status === "draft" ? "secondary" : "outline";
}

function DiffView({ diff }: { diff: GraphDiff }) {
  if (diff.identical) {
    return <p className="text-sm text-muted-foreground">No differences between these versions.</p>;
  }
  return (
    <div className="flex flex-col gap-3 text-sm">
      {diff.nodesAdded.length > 0 && (
        <section>
          <h3 className="font-medium text-success">Nodes added</h3>
          <ul className="ml-4 list-disc text-muted-foreground">
            {diff.nodesAdded.map((n) => <li key={n.id}>{n.label} ({n.kind})</li>)}
          </ul>
        </section>
      )}
      {diff.nodesRemoved.length > 0 && (
        <section>
          <h3 className="font-medium text-error">Nodes removed</h3>
          <ul className="ml-4 list-disc text-muted-foreground">
            {diff.nodesRemoved.map((n) => <li key={n.id}>{n.label} ({n.kind})</li>)}
          </ul>
        </section>
      )}
      {diff.nodesChanged.length > 0 && (
        <section>
          <h3 className="font-medium text-warning">Nodes changed</h3>
          <ul className="ml-4 list-disc text-muted-foreground">
            {diff.nodesChanged.map((n) => (
              <li key={n.id}>
                {n.label}: {n.changes.join("; ")}
              </li>
            ))}
          </ul>
        </section>
      )}
      {diff.edgesAdded.length > 0 && (
        <section>
          <h3 className="font-medium text-success">Edges added</h3>
          <ul className="ml-4 list-disc text-muted-foreground">
            {diff.edgesAdded.map((e) => <li key={e.id}>{e.source} → {e.target}</li>)}
          </ul>
        </section>
      )}
      {diff.edgesRemoved.length > 0 && (
        <section>
          <h3 className="font-medium text-error">Edges removed</h3>
          <ul className="ml-4 list-disc text-muted-foreground">
            {diff.edgesRemoved.map((e) => <li key={e.id}>{e.source} → {e.target}</li>)}
          </ul>
        </section>
      )}
    </div>
  );
}

/** Version list with two-version semantic comparison. */
export function VersionHistory({ workflowId }: { workflowId: string }) {
  const client = React.useMemo(
    () => createBrowserClient(process.env.NEXT_PUBLIC_DEV_USER ?? "local-dev"),
    [],
  );
  const [versions, setVersions] = React.useState<VersionSummary[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [selected, setSelected] = React.useState<number[]>([]);
  const [diff, setDiff] = React.useState<GraphDiff | null>(null);
  const [comparing, setComparing] = React.useState(false);

  React.useEffect(() => {
    client
      .listVersions(workflowId)
      .then(setVersions)
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "failed to load versions"),
      );
  }, [client, workflowId]);

  const toggle = (versionNumber: number) => {
    setDiff(null);
    setSelected((prev) => {
      if (prev.includes(versionNumber)) return prev.filter((v) => v !== versionNumber);
      return [...prev, versionNumber].slice(-2);
    });
  };

  const compare = async () => {
    if (selected.length !== 2) return;
    setComparing(true);
    setError(null);
    try {
      const [a, b] = [...selected].sort((x, y) => x - y);
      const [before, after]: [VersionDetail, VersionDetail] = await Promise.all([
        client.getVersion(workflowId, a),
        client.getVersion(workflowId, b),
      ]);
      setDiff(
        diffGraphs(before.graph as WorkflowGraph, after.graph as WorkflowGraph),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "comparison failed");
    } finally {
      setComparing(false);
    }
  };

  if (error && versions === null) {
    return (
      <div role="alert" className="rounded-lg border border-error bg-error-muted p-4 text-sm text-error-muted-foreground">
        Could not load versions: {error}
      </div>
    );
  }
  if (versions === null) {
    return <p className="text-sm text-muted-foreground">Loading versions…</p>;
  }
  if (versions.length === 0) {
    return <p className="text-sm text-muted-foreground">No versions yet.</p>;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="overflow-hidden rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead className="bg-muted text-left text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="px-3 py-2">Compare</th>
              <th className="px-3 py-2">Version</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">Author</th>
              <th className="px-3 py-2">Nodes / Edges</th>
              <th className="px-3 py-2">Hash</th>
              <th className="px-3 py-2">Created</th>
            </tr>
          </thead>
          <tbody>
            {versions.map((v) => (
              <tr key={v.versionNumber} className="border-t border-border">
                <td className="px-3 py-2">
                  <input
                    type="checkbox"
                    aria-label={`Select version ${v.versionNumber} for comparison`}
                    checked={selected.includes(v.versionNumber)}
                    onChange={() => toggle(v.versionNumber)}
                    className="h-4 w-4 accent-primary"
                  />
                </td>
                <td className="px-3 py-2 font-medium text-foreground">v{v.versionNumber}</td>
                <td className="px-3 py-2">
                  <Badge variant={statusVariant(v.status)}>{v.status}</Badge>
                </td>
                <td className="px-3 py-2 text-muted-foreground">{v.createdBy}</td>
                <td className="px-3 py-2 text-muted-foreground">{v.nodeCount} / {v.edgeCount}</td>
                <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                  {v.contentHash.slice(0, 12)}
                </td>
                <td className="px-3 py-2 text-muted-foreground">
                  {new Date(v.createdAt).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center gap-3">
        <Button
          size="sm"
          disabled={selected.length !== 2 || comparing}
          onClick={() => void compare()}
        >
          {comparing ? "Comparing…" : "Compare selected"}
        </Button>
        {diff && (
          <p className="text-sm text-muted-foreground">{summarizeDiff(diff)}</p>
        )}
        {error && <p className="text-sm text-error">{error}</p>}
      </div>

      {diff && (
        <div className="rounded-lg border border-border bg-card p-4">
          <DiffView diff={diff} />
        </div>
      )}
    </div>
  );
}
