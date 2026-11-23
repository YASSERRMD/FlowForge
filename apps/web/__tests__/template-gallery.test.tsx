import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TemplateGallery } from "@/components/workflows/template-gallery";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

const TEMPLATES = [
  {
    id: "invoice-approval",
    name: "Invoice approval",
    description: "Approve invoices under a threshold.",
    tags: ["finance", "approval"],
    source: "built-in",
    graph: {
      schemaVersion: "1",
      nodes: [
        { id: "start-1", kind: "start", label: "Start" },
        { id: "action-1", kind: "action", label: "Fetch invoice" },
        { id: "condition-1", kind: "condition", label: "Under threshold?" },
        { id: "end-1", kind: "end", label: "End" },
      ],
      edges: [],
    },
  },
  {
    id: "parallel-service-request",
    name: "Parallel service request validation",
    description: "Fork and join three checks.",
    tags: ["operations", "parallel"],
    source: "built-in",
    graph: { schemaVersion: "1", nodes: [], edges: [] },
  },
];

describe("TemplateGallery", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve(
          new Response(JSON.stringify({ templates: TEMPLATES }), { status: 200 }),
        ),
      ),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders templates with source labeling", async () => {
    render(<TemplateGallery />);
    expect(await screen.findByText("Invoice approval")).toBeInTheDocument();
    expect(screen.getByText("Parallel service request validation")).toBeInTheDocument();
    expect(screen.getAllByText("built-in")).toHaveLength(2);
  });

  it("filters by search query", async () => {
    render(<TemplateGallery />);
    await screen.findByText("Invoice approval");
    await userEvent.type(screen.getByLabelText("Search templates"), "parallel");
    expect(screen.queryByText("Invoice approval")).not.toBeInTheDocument();
    expect(screen.getByText("Parallel service request validation")).toBeInTheDocument();
  });

  it("expands a step preview", async () => {
    render(<TemplateGallery />);
    await screen.findByText("Invoice approval");
    await userEvent.click(screen.getByText("Preview 2 steps"));
    expect(screen.getByText("Fetch invoice")).toBeInTheDocument();
    expect(screen.getByText("Under threshold?")).toBeInTheDocument();
  });
});
