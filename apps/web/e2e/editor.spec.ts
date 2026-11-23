import { test, expect } from "@playwright/test";

// Builds and edits a simple graph against the real production bundle.
test.describe("workflow editor", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/workflows/demo");
  });

  // Node labels also appear in the validation inspector and config panel,
  // so canvas assertions scope to React Flow node elements.
  const canvasNode = (page: import("@playwright/test").Page, label: string) =>
    page.locator(".react-flow__node").getByText(label, { exact: true });

  test("canvas mounts with start and end nodes", async ({ page }) => {
    await expect(canvasNode(page, "Start")).toBeVisible();
    await expect(canvasNode(page, "End")).toBeVisible();
  });

  test("palette adds nodes with deterministic labels", async ({ page }) => {
    await page.getByRole("button", { name: "Add Action node" }).click();
    await page.getByRole("button", { name: "Add Action node" }).click();
    await page.getByRole("button", { name: "Add Condition node" }).click();

    await expect(canvasNode(page, "Action 1")).toBeVisible();
    await expect(canvasNode(page, "Action 2")).toBeVisible();
    await expect(canvasNode(page, "Condition 1")).toBeVisible();
    // Branch labels are visible on the condition node.
    await expect(canvasNode(page, "true")).toBeVisible();
    await expect(canvasNode(page, "false")).toBeVisible();
  });

  test("delete and undo restore a node via keyboard", async ({ page }) => {
    await page.getByRole("button", { name: "Add Action node" }).click();
    const node = canvasNode(page, "Action 1");
    await expect(node).toBeVisible();

    await node.click();
    await page.keyboard.press("Delete");
    await expect(node).not.toBeVisible();

    await page.keyboard.press("ControlOrMeta+z");
    await expect(canvasNode(page, "Action 1")).toBeVisible();
  });

  test("duplicate creates an independent copy", async ({ page }) => {
    await page.getByRole("button", { name: "Add Action node" }).click();
    await canvasNode(page, "Action 1").click();
    await page.keyboard.press("ControlOrMeta+d");
    // The copy keeps its label; there must now be two action nodes.
    await expect(page.locator(".react-flow__node-action")).toHaveCount(2);
    // Undo removes the copy.
    await page.keyboard.press("ControlOrMeta+z");
    await expect(page.locator(".react-flow__node-action")).toHaveCount(1);
  });

  test("start node cannot be deleted", async ({ page }) => {
    await page.keyboard.press("ControlOrMeta+a");
    await page.keyboard.press("Delete");
    await expect(canvasNode(page, "Start")).toBeVisible();
    await expect(canvasNode(page, "End")).toBeVisible();
  });
});
