# FlowForge execution status

- Current phase: 07 (gate: push/PR/merge pending)
- Current branch: phase/07-workflow-versioning-templates
- Repository remote: https://github.com/YASSERRMD/FlowForge.git
- Last verified merge to main: PR #6 `0133911` (phase 06, merge commit)
- Environment availability: Go [1.25.5], Node [24.11.1], package manager
  [pnpm 10.22.0], Docker [29.0.1], PostgreSQL [via Docker],
  Redis [via Docker], GitHub auth [gh 2.83.2, account YASSERRMD, scopes: repo]
- Current blocker: none

## Global acceptance evidence

- Example invoice approval [x] (phase 07 template, executable once phase 08 engine lands)
- Example parallel validation [x] (phase 07 template, executable once phase 08 engine lands)
- Example incident escalation retry [x] (phase 07 template, executable once phase 09 retry lands)
- Worker restart [ ] (phase 11)
- Security/IDOR checks [ ] (phase 13)
- All required CI [ ] (baseline added in phase 01, gates completed in phase 15)
- Clean checkout Compose startup [ ] (phase 15)
