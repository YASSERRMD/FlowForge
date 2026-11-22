import { test, expect } from "@playwright/test";

// Builds and edits a simple graph against the real production bundle.
test.describe("workflow editor", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/workflows/demo");
  });

  test("canvas mounts with start and end nodes", async ({ page }) => {
    await expect(page.getByText("Start", { exact: true })).toBeVisible();
    await expect(page.getByText("End", { exact: true })).toBeVisible();
  });

  test("palette adds nodes with deterministic labels", async ({ page }) => {
    await page.getByRole("button", { name: "Add Action node" }).click();
    await page.getByRole("button", { name: "Add Action node" }).click();
    await page.getByRole("button", { name: "Add Condition node" }).click();

    await expect(page.getByText("Action 1")).toBeVisible();
    await expect(page.getByText("Action 2")).toBeVisible();
    await expect(page.getByText("Condition 1")).toBeVisible();
    // Branch labels are visible on the condition node.
    await expect(page.getByText("true", { exact: true })).toBeVisible();
    await expect(page.getByText("false", { exact: true })).toBeVisible();
  });

  test("delete and undo restore a node via keyboard", async ({ page }) => {
    await page.getByRole("button", { name: "Add Action node" }).click();
    const node = page.getByText("Action 1");
    await expect(node).toBeVisible();

    await node.click();
    await page.keyboard.press("Delete");
    await expect(node).not.toBeVisible();

    await page.keyboard.press("ControlOrMeta+z");
    await expect(page.getByText("Action 1")).toBeVisible();
  });

  test("duplicate creates an independent copy", async ({ page }) => {
    await page.getByRole("button", { name: "Add Action node" }).click();
    await page.getByText("Action 1").click();
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
    await expect(page.getByText("Start", { exact: true })).toBeVisible();
    await expect(page.getByText("End", { exact: true })).toBeVisible();
  });
});
