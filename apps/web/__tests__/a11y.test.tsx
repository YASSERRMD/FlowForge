import { render } from "@testing-library/react";
import { axe } from "jest-axe";
import { describe, expect, it, vi } from "vitest";

import { Sidebar } from "@/components/layout/sidebar";
import { DataTable } from "@/components/data-table/data-table";
import { ErrorFallback } from "@/components/error-boundary";
import { FormField } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";
import { WorkflowStatusChip } from "@/components/workflows/workflow-status-chip";

vi.mock("next/navigation", () => ({
  usePathname: () => "/runs",
  redirect: () => {
    throw new Error("redirect");
  },
}));

describe("accessibility (axe)", () => {
  it("sidebar has no detectable violations", async () => {
    const { container } = render(<Sidebar />);
    expect(await axe(container)).toHaveNoViolations();
  });

  it("data table has no detectable violations", async () => {
    const { container } = render(
      <DataTable
        columns={[
          { id: "name", header: "Name", cell: (r: { name: string }) => r.name },
        ]}
        data={[{ name: "Alpha" }]}
        getRowId={(r) => r.name}
      />,
    );
    expect(await axe(container)).toHaveNoViolations();
  });

  it("error fallback has no detectable violations", async () => {
    const { container } = render(
      <ErrorFallback error={new Error("boom")} reset={() => {}} />,
    );
    expect(await axe(container)).toHaveNoViolations();
  });

  it("form field error state has no detectable violations", async () => {
    const { container } = render(
      <FormField label="Workflow name" error="Name is required">
        {(props) => <Input {...props} />}
      </FormField>,
    );
    expect(await axe(container)).toHaveNoViolations();
  });

  it("status chips have no detectable violations", async () => {
    const { container } = render(
      <div>
        <WorkflowStatusChip status="active" />
        <WorkflowStatusChip status="error" />
      </div>,
    );
    expect(await axe(container)).toHaveNoViolations();
  });
});

describe("form field accessibility wiring", () => {
  it("associates the error message with the input via aria-describedby", () => {
    const { getByLabelText, getByRole } = render(
      <FormField label="Workflow name" error="Name is required">
        {(props) => <Input {...props} />}
      </FormField>,
    );
    const input = getByLabelText("Workflow name");
    const alert = getByRole("alert");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAttribute("aria-describedby", alert.id);
  });
});
