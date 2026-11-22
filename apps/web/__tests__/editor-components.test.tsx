import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { NodePalette, DND_MIME } from "@/components/editor/palette";

describe("NodePalette", () => {
  it("renders every palette kind as a labelled button", () => {
    render(<NodePalette onAdd={() => {}} />);
    for (const label of ["Action", "Delay", "Condition", "Fork", "Join"]) {
      expect(
        screen.getByRole("button", { name: `Add ${label} node` }),
      ).toBeInTheDocument();
    }
  });

  it("clicking a palette button adds that node kind", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn();
    render(<NodePalette onAdd={onAdd} />);
    await user.click(screen.getByRole("button", { name: "Add Condition node" }));
    expect(onAdd).toHaveBeenCalledWith("condition");
  });

  it("keyboard activation (Enter) adds the node", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn();
    render(<NodePalette onAdd={onAdd} />);
    screen.getByRole("button", { name: "Add Fork node" }).focus();
    await user.keyboard("{Enter}");
    expect(onAdd).toHaveBeenCalledWith("fork");
  });

  it("drag start carries the node kind in dataTransfer", () => {
    render(<NodePalette onAdd={() => {}} />);
    const button = screen.getByRole("button", { name: "Add Delay node" });
    const data = new Map<string, string>();
    fireEvent.dragStart(button, {
      dataTransfer: {
        setData: (mime: string, value: string) => data.set(mime, value),
        effectAllowed: "",
        types: [],
      },
    });
    expect(data.get(DND_MIME)).toBe("delay");
  });
});
