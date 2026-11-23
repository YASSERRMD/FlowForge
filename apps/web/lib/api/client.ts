/**
 * Typed client for the FlowForge Go API (docs/api/openapi.yaml).
 * Hand-written to mirror the contract exactly; every error surface is the
 * uniform `{ error: { code, message, fields? } }` envelope.
 */

export type ApiErrorCode =
  | "internal_error"
  | "bad_request"
  | "malformed_json"
  | "body_too_large"
  | "unknown_fields"
  | "validation_failed"
  | "unauthorized"
  | "forbidden"
  | "not_found"
  | "conflict"
  | "service_unavailable";

export class ApiError extends Error {
  constructor(
    public readonly code: ApiErrorCode | "network_error",
    message: string,
    public readonly status: number,
    public readonly fields?: { field: string; message: string }[],
  ) {
    super(message);
    this.name = "ApiError";
  }

  get isConflict(): boolean {
    return this.code === "conflict";
  }

  get isUnauthorized(): boolean {
    return this.code === "unauthorized";
  }
}

export interface WorkflowSummary {
  id: string;
  name: string;
  description: string;
  status: "draft" | "active" | "archived";
  rowVersion: number;
  updatedAt: string;
}

export interface DraftState {
  versionId: string;
  versionNumber: number;
  rowVersion: number;
  contentHash: string;
  graph: unknown;
}

export interface WorkflowDetail extends WorkflowSummary {
  draft?: DraftState;
}

export interface VersionSummary {
  versionId: string;
  versionNumber: number;
  status: "draft" | "published" | "superseded";
  contentHash: string;
  createdBy: string;
  createdAt: string;
  publishedAt?: string;
  nodeCount: number;
  edgeCount: number;
}

export interface VersionDetail extends VersionSummary {
  graph: unknown;
}

export interface Template {
  id: string;
  name: string;
  description: string;
  tags: string[];
  source: "built-in";
  graph: unknown;
}

export interface PublishedVersion {
  versionId: string;
  versionNumber: number;
  contentHash: string;
  publishedAt: string;
}

export interface ClientOptions {
  baseUrl: string;
  /** Development-only identity header (APP_ENV=development, AUTH_DEV_MODE=true). */
  devUser?: string;
  fetchFn?: typeof fetch;
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  ifMatch?: string | number;
}

export class ApiClient {
  private readonly baseUrl: string;
  private readonly devUser?: string;
  private readonly fetchFn: typeof fetch;

  constructor(options: ClientOptions) {
    this.baseUrl = options.baseUrl.replace(/\/$/, "");
    this.devUser = options.devUser;
    // Bind the default fetch: calling window.fetch unbound throws
    // "Illegal invocation" in browsers.
    this.fetchFn = options.fetchFn ?? ((input, init) => fetch(input, init));
  }

  private async request<T>(path: string, options: RequestOptions = {}): Promise<{ data: T; etag: string | null }> {
    const headers: Record<string, string> = {};
    if (options.body !== undefined) headers["Content-Type"] = "application/json";
    if (this.devUser) headers["X-Dev-User"] = this.devUser;
    if (options.ifMatch !== undefined) {
      headers["If-Match"] =
        typeof options.ifMatch === "number" ? `"${options.ifMatch}"` : options.ifMatch;
    }

    let response: Response;
    try {
      response = await this.fetchFn(`${this.baseUrl}${path}`, {
        method: options.method ?? "GET",
        headers,
        body: options.body === undefined ? undefined : JSON.stringify(options.body),
      });
    } catch (err) {
      throw new ApiError(
        "network_error",
        err instanceof Error ? err.message : "network request failed",
        0,
      );
    }

    const etag = response.headers.get("ETag");
    if (response.status === 204) {
      return { data: undefined as T, etag };
    }

    const text = await response.text();
    let parsed: unknown = undefined;
    if (text !== "") {
      try {
        parsed = JSON.parse(text);
      } catch {
        throw new ApiError("internal_error", "server returned non-JSON response", response.status);
      }
    }

    if (!response.ok) {
      const envelope = parsed as {
        error?: { code?: string; message?: string; fields?: { field: string; message: string }[] };
      };
      throw new ApiError(
        (envelope?.error?.code as ApiErrorCode) ?? "internal_error",
        envelope?.error?.message ?? `request failed with status ${response.status}`,
        response.status,
        envelope?.error?.fields,
      );
    }
    return { data: parsed as T, etag };
  }

  async listWorkflows(options: { limit?: number; offset?: number } = {}): Promise<WorkflowSummary[]> {
    const params = new URLSearchParams();
    if (options.limit !== undefined) params.set("limit", String(options.limit));
    if (options.offset !== undefined) params.set("offset", String(options.offset));
    const query = params.size > 0 ? `?${params.toString()}` : "";
    const { data } = await this.request<{ workflows: WorkflowSummary[] }>(
      `/api/v1/workflows${query}`,
    );
    return data.workflows;
  }

  async listTemplates(): Promise<Template[]> {
    const { data } = await this.request<{ templates: Template[] }>("/api/v1/templates");
    return data.templates;
  }

  async createWorkflow(input: {
    name: string;
    description?: string;
    graph?: unknown;
    templateId?: string;
  }): Promise<{ workflow: WorkflowDetail; etag: string | null }> {
    const { data, etag } = await this.request<WorkflowDetail>("/api/v1/workflows", {
      method: "POST",
      body: input,
    });
    return { workflow: data, etag };
  }

  async getWorkflow(id: string): Promise<{ workflow: WorkflowDetail; etag: string | null }> {
    const { data, etag } = await this.request<WorkflowDetail>(`/api/v1/workflows/${id}`);
    return { workflow: data, etag };
  }

  async updateWorkflow(
    id: string,
    input: { name: string; description?: string },
  ): Promise<WorkflowSummary> {
    const { data } = await this.request<WorkflowSummary>(`/api/v1/workflows/${id}`, {
      method: "PATCH",
      body: input,
    });
    return data;
  }

  async deleteWorkflow(id: string): Promise<void> {
    await this.request<void>(`/api/v1/workflows/${id}`, { method: "DELETE" });
  }

  /** Duplicates a workflow with fresh entity IDs; the caller owns the clone. */
  async cloneWorkflow(id: string, name?: string): Promise<WorkflowDetail> {
    const { data } = await this.request<WorkflowDetail>(`/api/v1/workflows/${id}/clone`, {
      method: "POST",
      body: name ? { name } : {},
    });
    return data;
  }

  /**
   * Saves the draft graph. `ifMatch` is the draft rowVersion the caller last
   * read; the server answers 409 (ApiError.isConflict) when it is stale.
   */
  async publish(id: string): Promise<PublishedVersion> {
    const { data } = await this.request<PublishedVersion>(
      `/api/v1/workflows/${id}/publish`,
      { method: "POST" },
    );
    return data;
  }

  async listVersions(id: string): Promise<VersionSummary[]> {
    const { data } = await this.request<{ versions: VersionSummary[] }>(
      `/api/v1/workflows/${id}/versions`,
    );
    return data.versions;
  }

  async getVersion(id: string, versionNumber: number): Promise<VersionDetail> {
    const { data } = await this.request<VersionDetail>(
      `/api/v1/workflows/${id}/versions/${versionNumber}`,
    );
    return data;
  }

  /** Reinitializes the draft from a frozen version (If-Match guarded). */
  async copyVersionToDraft(
    id: string,
    versionNumber: number,
    ifMatch: number | string,
  ): Promise<{ draft: DraftState; etag: string | null }> {
    const { data, etag } = await this.request<DraftState>(
      `/api/v1/workflows/${id}/versions/${versionNumber}/copy-to-draft`,
      { method: "POST", ifMatch },
    );
    return { draft: data, etag };
  }

  async saveDraft(
    id: string,
    graph: unknown,
    ifMatch: number | string,
  ): Promise<{ draft: DraftState; etag: string | null }> {
    const { data, etag } = await this.request<DraftState>(`/api/v1/workflows/${id}/draft`, {
      method: "PUT",
      body: { graph },
      ifMatch,
    });
    return { draft: data, etag };
  }
}

/** Default client for the browser; base URL is same-origin or env override. */
export function createBrowserClient(devUser?: string): ApiClient {
  const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
  return new ApiClient({ baseUrl, devUser });
}
