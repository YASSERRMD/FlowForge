/**
 * Draft persistence: loads a workflow's draft into the editor and autosaves
 * changes with optimistic concurrency. Conflicts are surfaced, never
 * silently overwritten — autosave stops until the user resolves them.
 */
import { create } from "zustand";

import { ApiClient, ApiError, type WorkflowDetail } from "@/lib/api/client";
import type { WorkflowGraph } from "@/lib/contracts/graph";
import { countersFromNodes, fromWorkflowGraph, toWorkflowGraph } from "./convert";
import { useEditorStore } from "./store";

export type SaveStatus =
  | "idle"
  | "loading"
  | "saving"
  | "saved"
  | "error"
  | "conflict";

interface PersistenceState {
  workflowId: string | null;
  workflowName: string;
  rowVersion: number | null;
  status: SaveStatus;
  lastSavedAt: string | null;
  errorMessage: string | null;
}

export const usePersistenceStore = create<PersistenceState>(() => ({
  workflowId: null,
  workflowName: "",
  rowVersion: null,
  status: "idle",
  lastSavedAt: null,
  errorMessage: null,
}));

export const DEBOUNCE_MS = 800;

let timer: ReturnType<typeof setTimeout> | null = null;
let unsubscribe: (() => void) | null = null;
/** True while server state is being replayed into the editor store. */
let replaying = false;

function setState(partial: Partial<PersistenceState>) {
  usePersistenceStore.setState(partial);
}

function parseRowVersion(etag: string | null, fallback: number): number {
  if (!etag) return fallback;
  const n = Number(etag.replace(/"/g, ""));
  return Number.isInteger(n) && n > 0 ? n : fallback;
}

/** Loads a workflow's draft graph into the editor store. */
export async function loadWorkflow(client: ApiClient, id: string): Promise<WorkflowDetail> {
  setState({ status: "loading", workflowId: id, errorMessage: null });
  try {
    const { workflow, etag } = await client.getWorkflow(id);
    replaying = true;
    try {
      if (workflow.draft?.graph) {
        const { nodes, edges } = fromWorkflowGraph(workflow.draft.graph as WorkflowGraph);
        useEditorStore.setState({
          nodes,
          edges,
          counters: countersFromNodes(nodes) as never,
          past: [],
          future: [],
        });
      }
    } finally {
      replaying = false;
    }
    setState({
      status: "saved",
      workflowName: workflow.name,
      rowVersion: workflow.draft
        ? parseRowVersion(etag, workflow.draft.rowVersion)
        : null,
      lastSavedAt: null,
    });
    return workflow;
  } catch (err) {
    setState({
      status: "error",
      errorMessage: err instanceof ApiError ? err.message : "failed to load workflow",
    });
    throw err;
  }
}

async function saveNow(client: ApiClient) {
  const { workflowId, rowVersion, status } = usePersistenceStore.getState();
  if (!workflowId || rowVersion === null || status === "conflict") return;

  const { nodes, edges } = useEditorStore.getState();
  const graph = toWorkflowGraph(nodes, edges);
  setState({ status: "saving" });
  try {
    const { draft, etag } = await client.saveDraft(workflowId, graph, rowVersion);
    setState({
      status: "saved",
      rowVersion: parseRowVersion(etag, draft.rowVersion),
      lastSavedAt: new Date().toISOString(),
      errorMessage: null,
    });
  } catch (err) {
    if (err instanceof ApiError && err.isConflict) {
      // Never overwrite silently: stop autosaving until the user resolves.
      setState({
        status: "conflict",
        errorMessage: "This draft changed on the server since you loaded it.",
      });
      return;
    }
    if (err instanceof ApiError && err.status === 422) {
      setState({
        status: "error",
        errorMessage: `The server rejected this graph: ${err.message}`,
      });
      return;
    }
    setState({
      status: "error",
      errorMessage: err instanceof ApiError ? err.message : "save failed",
    });
  }
}

function scheduleSave(client: ApiClient) {
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => {
    timer = null;
    void saveNow(client);
  }, DEBOUNCE_MS);
}

/**
 * Starts autosaving editor changes for the loaded workflow.
 * Returns a stop function that cancels pending saves and unsubscribes.
 */
export function startAutosave(client: ApiClient): () => void {
  stopAutosave();
  let lastNodes = useEditorStore.getState().nodes;
  let lastEdges = useEditorStore.getState().edges;
  unsubscribe = useEditorStore.subscribe((state) => {
    if (replaying) return;
    if (state.nodes === lastNodes && state.edges === lastEdges) return;
    lastNodes = state.nodes;
    lastEdges = state.edges;
    scheduleSave(client);
  });
  return stopAutosave;
}

export function stopAutosave() {
  if (timer) {
    clearTimeout(timer);
    timer = null;
  }
  if (unsubscribe) {
    unsubscribe();
    unsubscribe = null;
  }
}

/**
 * Resolves a conflict by reloading the server draft, discarding local
 * edits made since the last successful save. Local undo history is reset
 * so stale snapshots cannot resurrect the conflicting state.
 */
export async function resolveConflictByReload(client: ApiClient): Promise<void> {
  const { workflowId } = usePersistenceStore.getState();
  if (!workflowId) return;
  await loadWorkflow(client, workflowId);
}

/** Forces an immediate save (flushes the debounce). */
export async function flushSave(client: ApiClient): Promise<void> {
  if (timer) {
    clearTimeout(timer);
    timer = null;
  }
  await saveNow(client);
}
