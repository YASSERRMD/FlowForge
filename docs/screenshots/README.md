# UI baseline screenshots

Screenshots are grouped per phase (`phase-02/`, ...). They are captured with
`apps/web/scripts/capture-screenshots.mjs` against a production build:

```bash
cd apps/web
pnpm build
pnpm start -p 3100 &
node scripts/capture-screenshots.mjs phase-02 http://localhost:3100
```

Each route is captured in light and dark themes; the workflows page is also
captured at tablet (768px) and mobile (390px) widths. These images are
baseline references for visual comparison in later phases, not test fixtures.
All data shown is static demo data, explicitly labelled in the UI.
