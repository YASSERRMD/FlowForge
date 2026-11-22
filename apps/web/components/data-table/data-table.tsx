"use client";

import * as React from "react";
import { ArrowUpDown, ChevronLeft, ChevronRight } from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export interface DataTableColumn<T> {
  id: string;
  header: string;
  cell: (row: T) => React.ReactNode;
  sortValue?: (row: T) => string | number;
  filterValue?: (row: T) => string;
  className?: string;
}

interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  data: T[];
  getRowId: (row: T) => string;
  filterPlaceholder?: string;
  pageSize?: number;
  loading?: boolean;
  emptyTitle?: string;
  emptyDescription?: string;
}

type SortState = { id: string; direction: "asc" | "desc" } | null;

export function DataTable<T>({
  columns,
  data,
  getRowId,
  filterPlaceholder = "Filter…",
  pageSize = 10,
  loading = false,
  emptyTitle = "No results",
  emptyDescription = "Nothing matches the current view.",
}: DataTableProps<T>) {
  const [sort, setSort] = React.useState<SortState>(null);
  const [filter, setFilter] = React.useState("");
  const [page, setPage] = React.useState(0);

  const filtered = React.useMemo(() => {
    const query = filter.trim().toLowerCase();
    if (!query) return data;
    return data.filter((row) =>
      columns.some((col) =>
        (col.filterValue?.(row) ?? "").toLowerCase().includes(query),
      ),
    );
  }, [data, columns, filter]);

  const sorted = React.useMemo(() => {
    if (!sort) return filtered;
    const col = columns.find((c) => c.id === sort.id);
    if (!col?.sortValue) return filtered;
    const direction = sort.direction === "asc" ? 1 : -1;
    return [...filtered].sort((a, b) => {
      const av = col.sortValue!(a);
      const bv = col.sortValue!(b);
      if (av === bv) return 0;
      return av > bv ? direction : -direction;
    });
  }, [filtered, columns, sort]);

  const pageCount = Math.max(1, Math.ceil(sorted.length / pageSize));
  const currentPage = Math.min(page, pageCount - 1);
  const pageRows = sorted.slice(
    currentPage * pageSize,
    currentPage * pageSize + pageSize,
  );

  function toggleSort(id: string) {
    setSort((prev) => {
      if (!prev || prev.id !== id) return { id, direction: "asc" };
      if (prev.direction === "asc") return { id, direction: "desc" };
      return null;
    });
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <Input
          value={filter}
          onChange={(event) => {
            setFilter(event.target.value);
            setPage(0);
          }}
          placeholder={filterPlaceholder}
          aria-label={filterPlaceholder}
          className="max-w-xs"
        />
        <p className="text-sm text-muted-foreground" aria-live="polite">
          {sorted.length} {sorted.length === 1 ? "row" : "rows"}
        </p>
      </div>

      <div className="rounded-md border border-border">
        <Table>
          <TableHeader>
            <TableRow>
              {columns.map((col) => (
                <TableHead key={col.id} className={col.className}>
                  {col.sortValue ? (
                    <button
                      type="button"
                      onClick={() => toggleSort(col.id)}
                      aria-sort={
                        sort?.id === col.id
                          ? sort.direction === "asc"
                            ? "ascending"
                            : "descending"
                          : undefined
                      }
                      className={cn(
                        "inline-flex items-center gap-1 rounded-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                        sort?.id === col.id && "text-foreground",
                      )}
                    >
                      {col.header}
                      <ArrowUpDown className="h-3.5 w-3.5" aria-hidden="true" />
                    </button>
                  ) : (
                    col.header
                  )}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={`skeleton-${i}`} aria-hidden="true">
                  {columns.map((col) => (
                    <TableCell key={col.id}>
                      <div className="h-4 w-3/4 animate-pulse rounded bg-muted" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : pageRows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="h-32 text-center"
                >
                  <p className="font-medium">{emptyTitle}</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {emptyDescription}
                  </p>
                </TableCell>
              </TableRow>
            ) : (
              pageRows.map((row) => (
                <TableRow key={getRowId(row)}>
                  {columns.map((col) => (
                    <TableCell key={col.id} className={col.className}>
                      {col.cell(row)}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-end gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={() => setPage((p) => Math.max(0, p - 1))}
          disabled={currentPage === 0}
          aria-label="Previous page"
        >
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          Previous
        </Button>
        <span className="text-sm text-muted-foreground" aria-live="polite">
          Page {currentPage + 1} of {pageCount}
        </span>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}
          disabled={currentPage >= pageCount - 1}
          aria-label="Next page"
        >
          Next
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  );
}
