import { describe, expect, it, vi } from "vitest";

import { ApiClient, ApiError } from "./client";

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}) {
  return new Response(body === undefined ? "" : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

function clientWith(handler: (url: string, init: RequestInit) => Response | Promise<Response>) {
  const fetchFn = vi.fn((input: RequestInfo | URL, init?: RequestInit) =>
    Promise.resolve(handler(String(input), init ?? {})),
  );
  const client = new ApiClient({
    baseUrl: "http://api.test/",
    devUser: "ada",
    fetchFn: fetchFn as unknown as typeof fetch,
  });
  return { client, fetchFn };
}

describe("ApiClient", () => {
  it("creates a workflow and exposes the ETag", async () => {
    const detail = {
      id: "wf-1",
      name: "My flow",
      description: "",
      status: "draft",
      rowVersion: 1,
      updatedAt: "2026-01-01T00:00:00Z",
      draft: { versionId: "v1", versionNumber: 1, rowVersion: 1, contentHash: "ab", graph: {} },
    };
    const { client, fetchFn } = clientWith(() =>
      jsonResponse(201, detail, { ETag: '"1"' }),
    );
    const { workflow, etag } = await client.createWorkflow({ name: "My flow" });
    expect(workflow.id).toBe("wf-1");
    expect(etag).toBe('"1"');
    const [url, init] = fetchFn.mock.calls[0];
    expect(url).toBe("http://api.test/api/v1/workflows");
    expect(init.method).toBe("POST");
    expect((init.headers as Record<string, string>)["X-Dev-User"]).toBe("ada");
  });

  it("saves a draft with If-Match quoted", async () => {
    const draft = { versionId: "v1", versionNumber: 1, rowVersion: 2, contentHash: "cd", graph: {} };
    const { client, fetchFn } = clientWith(() => jsonResponse(200, draft, { ETag: '"2"' }));
    const { draft: saved, etag } = await client.saveDraft("wf-1", { nodes: [] }, 1);
    expect(saved.rowVersion).toBe(2);
    expect(etag).toBe('"2"');
    const [, init] = fetchFn.mock.calls[0];
    expect((init.headers as Record<string, string>)["If-Match"]).toBe('"1"');
  });

  it("maps the error envelope to ApiError", async () => {
    const { client } = clientWith(() =>
      jsonResponse(409, {
        error: { code: "conflict", message: "the workflow changed since you loaded it" },
      }),
    );
    const err = await client.saveDraft("wf-1", {}, 1).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).code).toBe("conflict");
    expect((err as ApiError).status).toBe(409);
    expect((err as ApiError).isConflict).toBe(true);
  });

  it("surfaces field errors", async () => {
    const { client } = clientWith(() =>
      jsonResponse(400, {
        error: {
          code: "validation_failed",
          message: "request validation failed",
          fields: [{ field: "name", message: "is required" }],
        },
      }),
    );
    const err = await client.createWorkflow({ name: "" }).catch((e: unknown) => e);
    expect((err as ApiError).fields?.[0].field).toBe("name");
  });

  it("throws network_error when fetch rejects", async () => {
    const client = new ApiClient({
      baseUrl: "http://api.test",
      fetchFn: (() => Promise.reject(new TypeError("fetch failed"))) as unknown as typeof fetch,
    });
    const err = await client.listWorkflows().catch((e: unknown) => e);
    expect((err as ApiError).code).toBe("network_error");
    expect((err as ApiError).status).toBe(0);
  });

  it("handles 204 deletes", async () => {
    const { client } = clientWith(() => new Response("", { status: 204 }));
    await expect(client.deleteWorkflow("wf-1")).resolves.toBeUndefined();
  });

  it("builds list query params", async () => {
    const { client, fetchFn } = clientWith(() => jsonResponse(200, { workflows: [] }));
    await client.listWorkflows({ limit: 10, offset: 20 });
    expect(fetchFn.mock.calls[0][0]).toBe("http://api.test/api/v1/workflows?limit=10&offset=20");
  });
});
