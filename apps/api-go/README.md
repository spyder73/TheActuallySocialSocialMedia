# Go private pilot

This module supplies `api`, `migrate`, and `admin` executables. The current release supports invitation-only accounts and text social features. It does not yet replace legacy media, stories/snaps, or AI features. Deployment and recovery instructions are in [DEPLOY.md](../../docs/DEPLOY.md).

## Configuration

Set `DATABASE_URL` **or** `DATABASE_URL_FILE` (mounted secret), never both. `LISTEN_ADDR` defaults to `:8080`. The HTTP API requires `APP_ORIGIN`, an exact HTTPS browser origin with no trailing slash. Local HTTP requires `ALLOW_LOCAL_HTTP=true` and a loopback origin such as `http://localhost:18080`.

Optional HTTP settings are `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`, and `SHUTDOWN_TIMEOUT` (Go duration syntax). Handlers have a 15-second context deadline. `MIGRATIONS_DIR` overrides embedded SQL; normal containers use the embedded migrations. `api healthcheck` calls local liveness for minimal containers.

## Endpoints

All paths are relative to `/api`. Mutations require an `Origin` matching `APP_ORIGIN`, including login. Cookies are HttpOnly/SameSite=Lax and Secure outside explicit loopback development. Session token hashes are stored in PostgreSQL and checked on every authenticated request. Social responses are private and uncached.

| Route | Purpose |
| --- | --- |
| GET `/healthz`, `/readyz` | Public process/database probes |
| GET `/auth/me` | Current session identity |
| POST `/auth/login`, `/auth/logout` | Establish/revoke a session |
| POST `/auth/register` | Redeem an email-bound invitation |
| POST `/auth/reset` | Redeem a one-use password reset |
| GET `/feed?cursor=...` | Member-visible posts, newest first, 25 per page |
| GET/DELETE `/posts/{id}` | Read visible post / delete own post |
| POST `/posts` | Create text post; `visibility` is `public` (all members) or `close_friends` |
| GET/POST `/posts/{id}/comments` | Read/write visible post comments |
| GET `/members` | Active visible community identities |
| PATCH `/profile` | Update own display name and bio |
| GET `/relationships` | Own following/close-friend/blocked ID lists |
| PUT/DELETE `/relationships/{kind}/{id}` | `follow`, `close_friend`, or `block` |
| POST `/reports` | Report a post/user for operator review |
| GET/POST `/conversations` | List own conversations/create one with member IDs |
| GET/POST `/conversations/{id}/messages` | Member-only paginated text messages |

Errors return a JSON `error` string. Registration takes `token,email,username,password,displayName`; reset takes `token,password`. Post creation takes `text,visibility,parentPostId?`; conversation creation takes `memberIds,name?`; comment/message creation takes `text`. User identity objects exposed to other members exclude email and password hashes. `/auth/me` returns the account's own email and role. The original health OpenAPI document remains partial; a complete generated client is a subsequent contract milestone.

## Migration compatibility

The four files under `internal/migrate/migrations/prisma-baseline` are byte-for-byte copies of the original Prisma SQL. On adoption, each baseline migration must have one completed, non-rolled-back entry with a matching SHA-256 checksum. Unknown, incomplete, missing or altered history is rejected. Verified baseline rows enter `_tassm_go_migrations` without replaying SQL, then additive migrations under `migrations/go` run transactionally.

Empty databases receive the baseline first, then Go migrations. Populated databases without a recognized ledger are rejected. Go-ledger checksums are immutable; unknown migrations also prevent unsafe use of an older binary. The runner never drops or resets application data. Do not run Prisma and Go schema changes concurrently. Before adopting a legacy database, take a backup and rehearse restoration; schema compatibility does not imply feature parity with the original application.

## Tests

```sh
go test ./...
go vet ./...
# Use a DISPOSABLE PostgreSQL administrator URL. Integration tests create/drop isolated DBs.
TEST_DATABASE_URL=postgres://test:test@localhost:5432/postgres?sslmode=disable go test -race ./...
```

Integration tests cover baseline adoption, new migrations, invitation/session security and multi-user social authorization. The repository smoke script additionally exercises the API through Caddy on an isolated Docker project.
