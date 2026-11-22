/**
 * Captures baseline UI screenshots into docs/screenshots/<label>/.
 * Requires the app to be served already (default http://localhost:3100).
 *
 * Usage: node scripts/capture-screenshots.mjs [label] [baseURL]
 */
import { chromium } from "playwright";
import { mkdir } from "node:fs/promises";
import path from "node:path";

const label = process.argv[2] ?? "baseline";
const baseURL = process.argv[3] ?? "http://localhost:3100";
const outDir = path.resolve(
  new URL("../../../docs/screenshots", import.meta.url).pathname,
  label,
);

const shots = [
  { route: "/workflows", name: "workflows", viewport: { width: 1440, height: 900 } },
  { route: "/runs", name: "runs", viewport: { width: 1440, height: 900 } },
  { route: "/templates", name: "templates", viewport: { width: 1440, height: 900 } },
  { route: "/settings", name: "settings", viewport: { width: 1440, height: 900 } },
  { route: "/workflows", name: "workflows-tablet", viewport: { width: 768, height: 1024 } },
  { route: "/workflows", name: "workflows-mobile", viewport: { width: 390, height: 844 } },
];

await mkdir(outDir, { recursive: true });

const browser = await chromium.launch();
try {
  for (const theme of ["light", "dark"]) {
    for (const shot of shots) {
      const page = await browser.newPage({ viewport: shot.viewport });
      await page.addInitScript((t) => {
        window.localStorage.setItem("theme", t);
      }, theme);
      await page.goto(`${baseURL}${shot.route}`, { waitUntil: "networkidle" });
      const file = path.join(outDir, `${shot.name}-${theme}.png`);
      await page.screenshot({ path: file, fullPage: true });
      console.log(`captured ${file}`);
      await page.close();
    }
  }
} finally {
  await browser.close();
}
