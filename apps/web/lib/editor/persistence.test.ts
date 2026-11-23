import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiClient, ApiError } from "@/lib/api/client";
import {
  DEBOUNCE_MS,
  flushSave,
  loadWorkflow,
  resolveConflictByReload,
  startAutosave,
  stopAutosave,
  usePersistenceStore,
} from "./persistence";
import { useEditorStore } from "./store";

const VALID_GRAPH = {
  schemaVersion: "1",
  nodes: [
    { id: "start-1", kind: "start", label: "Start" },
    { id: "end-1", kind: "end", label: "End" },
  ],
  edges: [{ id: "e-1", source: "start-1", target: "end-1" }],
};

function fakeClient(handlers: {
  get?: (id: string) => Response;
  save?: (id: string, ifMatch: string) => Response;
}): ApiClient {
  const fetchFn = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = String(input);
    if (init?.method === "PUT" && handlers.save) {
      return Promise.resolve(handlers.save(url, (init.headers as Record<string, string>)["If-Match"]));
    }
    if (handlers.get) return Promise.resolve(handlers.get(url));
    return Promise.resolve(new Response(null, { status: 404 }));
  };
  return new ApiClient({ baseUrl: "http://api.test", fetchFn: fetchFn as typeof fetch });
}

function workflowBody(graph: unknown = VALID_GRAPH, rowVersion = 1) {
  return {
    id: "wf-1",
    name: "My flow",
    description: "",
    status: "draft",
    rowVersion: 1,
    updatedAt: "2026-01-01T00:00:00Z",
    draft: {
      versionId: "v-1",
      versionNumber: 1,
      rowVersion,
      contentHash: "ab",
      graph,
    },
  };
}

describe("persistence", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    usePersistenceStore.setState({
      workflowId: null,
      workflowName: "",
      rowVersion: null,
      status: "idle",
      lastSavedAt: null,
      errorMessage: null,
    });
    useEditorStore.setState({ past: [], future: [] });
  });

  afterEach(() => {
    stopAutosave();
    vi.useRealTimers();
  });

  it("loads a draft into the editor store", async () => {
    const client = fakeClient({
      get: () =>
        new Response(JSON.stringify(workflowBody()), {
          status: 200,
          headers: { ETag: '"1"' },
        }),
    });
    await loadWorkflow(client, "wf-1");
    const state = useEditorStore.getState();
    expect(state.nodes.map((n) => n.id).sort()).toEqual(["end-1", "start-1"]);
    expect(state.edges).toHaveLength(1);
    expect(usePersistenceStore.getState()).toMatchObject({
      status: "saved",
      rowVersion: 1,
      workflowName: "My flow",
    });
  });

  it("autosaves debounced editor changes and advances the token", async () => {
    const saves: string[] = [];
    const client = fakeClient({
      get: () => new Response(JSON.stringify(workflowBody()), { status: 200, headers: { ETag: '"1"' } }),
      save: (_id, ifMatch) => {
        saves.push(ifMatch);
        return new Response(
          JSON.stringify({ versionId: "v-1", versionNumber: 1, rowVersion: 2, contentHash: "cd", graph: VALID_GRAPH }),
          { status: 200, headers: { ETag: '"2"' } },
        );
      },
    });
    await loadWorkflow(client, "wf-1");
    startAutosave(client);

    useEditorStore.getState().addNode("action", { x: 100, y: 100 });
    useEditorStore.getState().addNode("delay", { x: 100, y: 200 });
    await vi.advanceTimersByTimeAsync(DEBOUNCE_MS + 10);

    // Two rapid edits collapse into one save.
    expect(saves).toEqual(['"1"']);
    expect(usePersistenceStore.getState()).toMatchObject({ status: "saved", rowVersion: 2 });
  });

  it("stops autosaving on conflict until the user reloads", async () => {
    let saveCalls = 0;
    const client = fakeClient({
      get: () => new Response(JSON.stringify(workflowBody()), { status: 200, headers: { ETag: '"5"' } }),
      save: () => {
        saveCalls++;
        return new Response(
          JSON.stringify({ error: { code: "conflict", message: "stale" } }),
          { status: 409 },
        );
      },
    });
    await loadWorkflow(client, "wf-1");
    startAutosave(client);

    useEditorStore.getState().addNode("action", { x: 0, y: 0 });
    await vi.advanceTimersByTimeAsync(DEBOUNCE_MS + 10);
    expect(usePersistenceStore.getState().status).toBe("conflict");

    // Further edits do not trigger more saves while conflicted.
    useEditorStore.getState().addNode("delay", { x: 0, y: 100 });
    await vi.advanceTimersByTimeAsync(DEBOUNCE_MS * 3);
    expect(saveCalls).toBe(1);

    await resolveConflictByReload(client);
    expect(usePersistenceStore.getState()).toMatchObject({ status: "saved", rowVersion: 5 });
  });

  it("flushes pending saves immediately", async () => {
    let saveCalls = 0;
    const client = fakeClient({
      get: () => new Response(JSON.stringify(workflowBody()), { status: 200, headers: { ETag: '"1"' } }),
      save: () => {
        saveCalls++;
        return new Response(
          JSON.stringify({ versionId: "v-1", versionNumber: 1, rowVersion: 2, contentHash: "cd", graph: VALID_GRAPH }),
          { status: 200, headers: { ETag: '"2"' } },
        );
      },
    });
    await loadWorkflow(client, "wf-1");
    startAutosave(client);
    useEditorStore.getState().addNode("action", { x: 0, y: 0 });
    await flushSave(client);
    expect(saveCalls).toBe(1);
  });

  it("surfaces non-conflict errors without losing state", async () => {
    const client = fakeClient({
      get: () => new Response(JSON.stringify(workflowBody()), { status: 200, headers: { ETag: '"1"' } }),
      save: () =>
        new Response(JSON.stringify({ error: { code: "service_unavailable", message: "db down" } }), { status: 503 }),
    });
    await loadWorkflow(client, "wf-1");
    startAutosave(client);
    useEditorStore.getState().addNode("action", { x: 0, y: 0 });
    await vi.advanceTimersByTimeAsync(DEBOUNCE_MS + 10);
    const state = usePersistenceStore.getState();
    expect(state.status).toBe("error");
    expect(state.errorMessage).toBe("db down");
    // Node edits are still in the editor store.
    expect(useEditorStore.getState().nodes.some((n) => n.type === "action")).toBe(true);
  });

  it("maps ApiError conflicts via isConflict", () => {
    const err = new ApiError("conflict", "stale", 409);
    expect(err.isConflict).toBe(true);
  });
});
