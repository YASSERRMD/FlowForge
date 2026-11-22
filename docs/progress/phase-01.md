# Phase 01 — Repository and engineering foundation

- Branch: `phase/01-repository-foundation`
- Objective: establish auditable repository conventions, architecture decisions,
  reproducible local tooling and the initial monorepo structure.

## Task journal

| Task | Scope | Verification | Commit SHA | Status |
|---|---|---|---|---|
| 01.01 create repository layout | monorepo dirs + READMEs | dir listing | pending | in progress |
| 01.02 document architectural boundaries | docs/architecture.md | review | pending | pending |
| 01.03 define engineering principles | docs/adr/0001-0005 | review | pending | pending |
| 01.04 configure git safeguards | .gitignore/.gitattributes/.editorconfig | git check-ignore | pending | pending |
| 01.05 bootstrap go workspaces | go.mod, cmd/api, cmd/worker | go build ./... | pending | pending |
| 01.06 bootstrap nextjs frontend | apps/web | pnpm build | pending | pending |
| 01.07 add local configuration samples | .env.example + env validation | unit test | pending | pending |
| 01.08 add reproducible tool commands | Makefile | make -n targets | pending | pending |
| 01.09 add CI baseline | .github/workflows/ci.yml | yaml parse | pending | pending |
| 01.10 write developer onboarding | docs/development.md | review | pending | pending |
| 01.11 validate empty-project build | gate checks + phase report | go/pnpm builds | pending | pending |

## Phase closure evidence

To be completed at the end of the phase with real command outputs.
