# db

- `queries/` — sqlc SQL query definitions that generate the typed Go data layer.
- Schema migrations live in `internal/db/migrations/` and are embedded into
  the binaries (`internal/db/migrations.go`), so deployed services apply
  schema changes without external files. Apply them with
  `go run ./apps/api migrate`.
