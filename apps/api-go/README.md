# Go API foundation

This module is the in-progress replacement API foundation. It currently provides `api` and `migrate` executables, bounded HTTP settings, request IDs, JSON logs, and liveness/readiness probes. It does not implement accounts, sessions, or social routes.

## Configuration

`DATABASE_URL` is required. Docker deployments may set `DATABASE_URL_FILE` to a mounted secret instead; the two settings are mutually exclusive. `LISTEN_ADDR` defaults to `:8080`. Optional HTTP timeouts are `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`, and `SHUTDOWN_TIMEOUT` (Go duration syntax). `MIGRATIONS_DIR` can point to a mounted SQL migration directory; otherwise migrations are embedded in the binary.

The API has `GET /api/healthz` for process liveness and `GET /api/readyz` for PostgreSQL readiness. The `api healthcheck` subcommand checks the local liveness endpoint, intended for minimal containers without curl.

## Existing Prisma database strategy

The four SQL files under `internal/migrate/migrations/prisma-baseline` are byte-for-byte copies of these `apps/api/prisma/migrations/<name>/migration.sql` files from the current Node backend: `20260930171014_init`, `20260930200528_ai_conversations`, `20260930204856_replies_comments_closefriends_moderation_groups`, and `20261003145943_media_asset_ownership`. They are embedded and SHA-256 checked against `_prisma_migrations` before the Go command adopts an existing database. Adoption requires every packaged migration to have one completed, non-rolled-back ledger entry with a matching checksum; unknown, incomplete, missing, or checksum-mismatched history stops with an error. It then records the verified baseline in `_tassm_go_migrations` without rerunning SQL.

For a database without either ledger, the command applies the packaged SQL only when the `public` schema has no application tables. A populated database without a recognized ledger is left untouched and rejected. Prisma deployments that contain migrations beyond this packaged baseline are also rejected until their SQL is deliberately incorporated; additional Go migrations currently apply only to fresh databases owned by the Go ledger. The runner never drops or resets schema/data. After adoption, do not run Node/Prisma schema changes and Go migrations concurrently; the Node API remains the active application during this foundation phase, and migration ownership must be coordinated during cutover.

To verify migration copy provenance, compare each file with its corresponding `apps/api/prisma/migrations/<migration-name>/migration.sql`; the filenames retain the Prisma migration names. On Go-ledger databases, additional SQL files are ordered lexically, recorded with checksums, and applied transactionally. In Docker, copy the `internal/migrate/migrations` contents into `/app/migrations` and set `MIGRATIONS_DIR=/app/migrations`.

## Local checks

From this directory, run `go test ./...` and `go vet ./...`. To compile container binaries for Linux/amd64:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /private/tmp/tassm-api ./cmd/api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /private/tmp/tassm-migrate ./cmd/migrate
```
