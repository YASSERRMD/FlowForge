# Phase 02 — Product design system and frontend shell

Branch: `phase/02-design-system-ui-shell` (from merged `origin/main` after PR #1)

## Task journal

| # | Task | Commit | Evidence |
|---|------|--------|----------|
| 02.01 | Add design tokens | `7c86d8e` | `app/globals.css` `@theme` tokens: type scale, radius, light/dark palettes, semantic success/warning/error/info; `pnpm build` passes |
| 02.02 | Install accessible ui primitives | `9ba6ba7` | Radix-based button, dialog, dropdown-menu, tabs, tooltip, label, input + `cn()` util; lint/tsc/build pass |
| 02.03 | Build application navigation | `ca9cf80` | `TopNav` (mobile dialog menu), `Sidebar` (`aria-current`, labelled landmark), `AppShell` with skip link |
| 02.04 | Create dashboard frame | `5081531` | `(dashboard)` route group with `/workflows`, `/runs`, `/templates`, `/settings`; `/` redirects to `/workflows`; all routes prerender in build output |
| 02.05 | Implement theme preference | `3841dc6` | `next-themes` provider (light/dark/system, `localStorage` persistence), `ThemeToggle` dropdown in top nav |
| 02.06 | Build reusable data tables | `de62e29` | Generic `DataTable`: sortable (`aria-sort` on `th`), filterable, paginated, skeleton loading, accessible empty state |
| 02.07 | Build workflow listing ui | `5b0d497` | Card/list views, search, status tabs, owner badges, status chips; explicit "Demo data" notice |
| 02.08 | Add run overview components | `2d5578f` | Summary cards + dependency-free SVG trend chart; demo banner on `/runs` |
| 02.09 | Implement toast and error ux | `01c9e45` | `sonner` Toaster (theme-aware), route `error.tsx` boundary, reusable `ErrorBoundary`/`ErrorFallback`, accessible `FormField` with `aria-invalid`/`aria-describedby` error wiring |
| 02.10 | Add responsive accessibility tests | `466271d` | vitest + Testing Library + jest-axe: 38 tests — sorting/filter/pagination, keyboard focus order, skip link, axe scans, WCAG AA token contrast for both palettes. Two real contrast failures found and fixed in tokens |
| 02.11 | Polish layout and screenshots | `3eb3319` | 12 baseline screenshots in `docs/screenshots/phase-02/` (desktop/tablet/mobile × light/dark) via Playwright capture script; captures verified visually |

## Phase gate results (run on HEAD `3eb3319`)

- `pnpm lint` — clean
- `pnpm exec tsc --noEmit` — clean
- `pnpm test` — 38/38 passing (4 files)
- `pnpm build` — all 6 routes prerender
- `go vet ./...`, `go test ./...`, `go test -race ./...`, `go build ./...` — pass (unchanged Go code)
- `git diff --check` — clean
- Commit subjects: 11 × `phase 02: ...`
- Author/committer: `YASSERRMD <arafath.yasser@gmail.com>` on all commits (verified via `git log`)
- No secrets committed (CI git-hygiene job)

## Notes

- No API integration exists yet; all listing/metrics surfaces show static demo
  data and are explicitly labelled as such in the UI.
- CI fix commits from phase 01 (`a6c514f`, `10e852e`) are on main; phase 02
  required no CI changes.
