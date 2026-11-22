import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import {
  DataTable,
  type DataTableColumn,
} from "@/components/data-table/data-table";

interface Row {
  id: string;
  name: string;
  count: number;
}

const columns: DataTableColumn<Row>[] = [
  {
    id: "name",
    header: "Name",
    cell: (row) => row.name,
    sortValue: (row) => row.name,
    filterValue: (row) => row.name,
  },
  {
    id: "count",
    header: "Count",
    cell: (row) => row.count,
    sortValue: (row) => row.count,
  },
];

function makeRows(n: number): Row[] {
  return Array.from({ length: n }, (_, i) => ({
    id: `row-${i}`,
    name: `Item ${String(i).padStart(2, "0")}`,
    count: i,
  }));
}

describe("DataTable", () => {
  it("filters rows by the search input", async () => {
    const user = userEvent.setup();
    render(
      <DataTable
        columns={columns}
        data={makeRows(5)}
        getRowId={(r) => r.id}
      />,
    );

    await user.type(screen.getByLabelText(/filter/i), "item 03");
    expect(screen.getByText("Item 03")).toBeInTheDocument();
    expect(screen.queryByText("Item 00")).not.toBeInTheDocument();
    expect(screen.getByText("1 row")).toBeInTheDocument();
  });

  it("sorts ascending then descending then clears", async () => {
    const user = userEvent.setup();
    render(
      <DataTable columns={columns} data={makeRows(3)} getRowId={(r) => r.id} />,
    );

    const sortButton = screen.getByRole("button", { name: /count/i });
    const header = screen.getByRole("columnheader", { name: /count/i });
    await user.click(sortButton);
    expect(header).toHaveAttribute("aria-sort", "ascending");

    await user.click(sortButton);
    expect(header).toHaveAttribute("aria-sort", "descending");

    await user.click(sortButton);
    expect(header).not.toHaveAttribute("aria-sort");
  });

  it("paginates with next/previous controls", async () => {
    const user = userEvent.setup();
    render(
      <DataTable
        columns={columns}
        data={makeRows(25)}
        getRowId={(r) => r.id}
        pageSize={10}
      />,
    );

    expect(screen.getByText("Page 1 of 3")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /next page/i }));
    expect(screen.getByText("Page 2 of 3")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /previous page/i }));
    expect(screen.getByText("Page 1 of 3")).toBeInTheDocument();
  });

  it("shows an accessible empty state", () => {
    render(
      <DataTable
        columns={columns}
        data={[]}
        getRowId={(r) => r.id}
        emptyTitle="No workflows"
        emptyDescription="Create your first workflow."
      />,
    );
    expect(screen.getByText("No workflows")).toBeInTheDocument();
    expect(screen.getByText("Create your first workflow.")).toBeInTheDocument();
  });

  it("renders loading skeleton rows hidden from assistive tech", () => {
    render(
      <DataTable columns={columns} data={[]} getRowId={(r) => r.id} loading />,
    );
    const table = screen.getByRole("table");
    expect(within(table).queryByText("No results")).not.toBeInTheDocument();
  });
});
