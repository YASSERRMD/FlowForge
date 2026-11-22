# apps/api

Go HTTP + WebSocket API service for FlowForge. chi v5 router, pgx v5 with
sqlc-generated typed queries, PostgreSQL as the only source of truth. Serves
`/api/v1`, `/healthz`, `/readyz` and authenticated run event streams.
