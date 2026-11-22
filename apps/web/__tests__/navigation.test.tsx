import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AppShell } from "@/components/layout/app-shell";
import { Sidebar } from "@/components/layout/sidebar";

vi.mock("next/navigation", () => ({
  usePathname: () => "/workflows",
  redirect: () => {
    throw new Error("redirect");
  },
}));

describe("navigation", () => {
  it("marks the active section with aria-current", () => {
    render(<Sidebar />);
    const workflows = screen.getByRole("link", { name: /workflows/i });
    expect(workflows).toHaveAttribute("aria-current", "page");
    const runs = screen.getByRole("link", { name: /runs/i });
    expect(runs).not.toHaveAttribute("aria-current");
  });

  it("exposes a labelled primary navigation landmark", () => {
    render(<Sidebar />);
    expect(
      screen.getByRole("navigation", { name: /primary/i }),
    ).toBeInTheDocument();
  });

  it("provides a skip link as the first focusable element", async () => {
    const user = userEvent.setup();
    render(
      <AppShell>
        <p>content</p>
      </AppShell>,
    );
    await user.tab();
    expect(screen.getByRole("link", { name: /skip to content/i })).toHaveFocus();
  });

  it("moves keyboard focus through nav links in order", async () => {
    const user = userEvent.setup();
    render(<Sidebar />);
    const links = screen.getAllByRole("link");
    for (const link of links) {
      await user.tab();
      expect(link).toHaveFocus();
    }
  });

  it("has a labelled mobile menu trigger", () => {
    render(
      <AppShell>
        <p>content</p>
      </AppShell>,
    );
    expect(
      screen.getByRole("button", { name: /open navigation menu/i }),
    ).toBeInTheDocument();
  });
});
