# Private text pilot release

The Go API now serves invitation accounts, revocable sessions, profiles, chronological text posts, comments/replies, close friends, follows, bilateral blocking, reports, and private/group text conversations. The dark/light/system frontend uses the real API. The fictional design preview remains isolated at `/preview` and is excluded from ordinary production builds.

Start with [DEPLOY.md](DEPLOY.md) for VPS setup, owner invitations, HTTPS, moderation, encrypted backups, restoration and upgrades. No public VPS has been deployed from this workspace.

## Local startup

```sh
# Fresh checkout only: bootstrap refuses to overwrite existing secrets.
sh scripts/bootstrap-pilot.sh --local
docker compose --env-file .env.pilot -f compose.pilot.yaml up -d --build --wait
```

Open `http://localhost:18080`. The installation starts empty; create an owner invitation using the deployment guide. Use `localhost` consistently because mutation origins are checked. Existing foundation checkouts must add `APP_ORIGIN=http://localhost:18080` and `ALLOW_LOCAL_HTTP=true` to their private `.env.pilot` before starting this release. Do not regenerate existing database secrets.

## Verified locally

- Go unit/race/vet checks and disposable PostgreSQL integration tests passed. Tests include invitation replay, password compatibility/bounds, logout/reset revocation, strict origins, schema adoption and checksums, restricted posts and recursive private ancestors, bilateral blocks, and conversation membership.
- Docker images build and start with PostgreSQL → migrations → API → Caddy readiness ordering. A three-account smoke script exercises the actual HTTP routes through Caddy, including admin invitations, reset, and disabling accounts.
- An age-encrypted snapshot was restored into a new database; deterministic hashes confirmed every row in all 20 public tables matched before subsequent test writes.
- The live browser was checked at desktop and phone widths, in dark/light modes, for sign-in, posting, comments, and direct messages. The invitation-route regression found during review was fixed and checked separately. Default production builds exclude preview assets.
- CI runs the Go integration suite, frontend builds, container smoke test, and encrypted backup/restore comparison.

## Scope still pending

This is a **text-first private pilot**, not full parity with the original application. Media uploads/private storage, stories/snaps, AI chats, live Alethea checks, durable checking jobs and SSE remain future milestones. Messages have a manual refresh button. Comments show up to 500 entries, and conversation lists up to 200; the pilot is designed for 2–10 people.

The original Node backend remains in the repository. Do not cut over an existing community relying on its media or AI features yet. The OpenAPI file still covers only probes; the complete generated client is pending. Public DNS/TLS, a real VPS, and scheduled off-host backup delivery need operational verification when a host/domain are supplied.
