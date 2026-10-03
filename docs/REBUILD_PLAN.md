# TASSM rebuild plan

Status: Go/container foundation and isolated frontend preview implemented; private pilot still in progress.
Date: 2026-10-03.

## Confirmed direction

- React/TypeScript frontend with a complete visual redesign, dark/light/system themes.
- Go backend replacing the current Fastify API.
- Integrate the Go fact-checking engine from spyder73/factcheckerweb (Alethea).
- Invitation-only pilot for 2–10 people on one small Linux VPS.
- Standard secure accounts: HTTPS, hashed passwords and protected sessions. End-to-end message encryption is outside this pilot.
- Preserve chronological feeds, no ads, user-directed AI, close-friend visibility and existing social features.
- Use an orchestrator with focused subagents; keep shared architecture and integration under one owner.

Reviewed baselines: TASSM `48a6e6f8c3c8695eea0113ff43de50a2f6c36088`; Alethea `6bcf6eada71bf199752c5a730fa60bfd14c98d68`.

## Architecture

Browser → Caddy (HTTPS, static React assets) → Go social API → PostgreSQL/private media storage.
The Go API dispatches durable fact-check jobs to an internal Alethea service; the browser receives progress through the social API.

| Component | Decision |
| --- | --- |
| Web | React, TypeScript, Vite, TanStack Query; semantic Tailwind theme tokens and accessible UI primitives |
| API | Go, standard HTTP/chi, pgx; explicit SQL migrations, optionally sqlc for typed queries |
| Contract | OpenAPI with a generated TypeScript client; contract owned by orchestrator |
| Realtime | HTTP mutations plus authenticated SSE for message notifications/check progress initially; reconnect reconciles state through HTTP |
| Persistence | PostgreSQL; keep current data/IDs and compatible password hashes |
| Media | Private S3-compatible storage behind an adapter; retain SeaweedFS initially with real credentials and a pinned image |
| Jobs | PostgreSQL-backed queue with leases, bounded retries, idempotency and restart recovery |
| Fact-checking | Internal service-authenticated Alethea; reuse its Redis dependency only if needed by the integrated path |
| Deployment | Docker Compose, multistage builds, Caddy, persistent volumes, health checks and encrypted off-host backups |

SSE is sufficient for the current notification behavior; Socket.IO must be replaced in both client and server deliberately. Add WebSockets only for features requiring them. API/database/queue changes should remain a modular application rather than a collection of independently deployed social services.

Alethea stays an optional deployment profile so ordinary social use works without AI/search credentials or during an outage. External provider keys are needed to perform live checks. No local LLM hosting is assumed on the small VPS.

## Visual direction: After Hours Editorial

A serious private social space with the typography and composition of a contemporary journal.

| Element | Initial design specification |
| --- | --- |
| Dark | Ink canvas #0D1117, slate surfaces #151B23, warm foreground #ECEEEA, restrained teal #85C9BE |
| Light | Warm paper #F5F4EF, pale surfaces, charcoal text, deeper teal #246C62 |
| Type | Self-hosted readable sans for UI/body; editorial serif used sparingly for large page headings; verify font licenses |
| Desktop | About 208px navigation, 660–720px feed, optional 280–320px contextual rail; collapse based on available space |
| Mobile | Full-width feed, bottom navigation, single-view messaging, full-height evidence sheet, safe-area support |
| Motion | Roughly 120–200ms control feedback and 220–280ms panel transitions; respect reduced motion |
| Signature | Chronological date markers, expressive headings, deliberate image framing, calm caught-up ending |

Colors are starting points, subject to contrast testing. All components use semantic tokens; theme preference persists without an initial flash. Keep German copy consistent with the existing product initially and centralize labels for later localization.

The first visual deliverable is a working feed → post → evidence experience in both themes, with representative fixture content confined to development/demo mode. The real installation starts empty. Small-community states must feel intentional; do not fabricate trending content, activity or popularity counts.

The evidence panel shows the exact claim, finding, source passages and dates, contrary evidence, limitations, and check version. The feed shows a compact summary. Comments and AI conversations open on demand rather than expanding inside every post.

Visual acceptance: inspect desktop/tablet/360px phone layouts in both themes; no horizontal overflow; readable long posts/usernames; keyboard focus and dialog focus handling; 44px touch targets; reduced motion; stable image loading; complete empty/loading/error states. Keep source code details out of user-facing screens.

## Secure private pilot

- All community content requires membership by default. Existing “public” visibility means all members for this deployment, not anonymous internet access; label it accordingly in the UI.
- Admin CLI bootstraps the owner and creates one-use expiring invitations. No shared or seeded default password.
- Passwords use Argon2id, with bounded resource use and compatibility fixtures for existing hashes.
- Random opaque session tokens in Secure/HttpOnly/SameSite cookies; store only token hashes server-side. Logout, account disabling and password resets revoke sessions.
- Same-origin hosting, CSRF/origin checks, login/invite/reset throttling, one-use reset links and generic login failures.
- Admin-generated reset links avoid requiring an email provider for the pilot. Never log tokens or API keys.
- Preserve authenticated media reads and authorization checks for follows, close friends, blocks, conversation membership, snap recipients and expiry.
- Encrypt stored provider credentials with authenticated encryption; retain existing format compatibility or provide a tested migration.
- HTTPS protects transport; passwords are hashed; backups are encrypted. Posts/messages remain readable by the server and must not be described as end-to-end encrypted.
- Minimal admin operations: invite, disable account, revoke sessions, issue reset link and review/remove reported content.

## Delivery sequence and gates

### 1. Contract and container foundation

Inventory existing routes and authorization policies. Establish an OpenAPI contract, Go skeleton, migration baseline, readiness endpoints and CI. Add the production Compose structure, Caddy routing and reproducible frontend/API images immediately.

Gate: a clean checkout can build and start the container skeleton; API health/readiness and database migrations behave predictably. Preserve existing schema migration history; do not silently replace the database.

### 2. Visual proof and secure login

In parallel, build the shared AppShell/theme system and the feed/post/evidence visual proof against explicit fixtures. Implement Go invitation redemption, login/logout, sessions, profiles and the common authorization layer. Connect the new login flow to the frontend.

Gate: owner can create an invite; two people can sign in independently; logout revokes access. Feed/evidence works interactively in dark and light modes at phone and desktop widths. The visual checkpoint permits early feedback without blocking unrelated backend work.

### 3. Core social migration

Port feed pagination/caught-up state, posts, uploads, replies/comments, follows, close friends, blocking/reporting, profiles and direct/group conversations. Port stories/snaps and durable expiry cleanup before retiring the Node API. Update realtime clients intentionally; preserve feature parity and improve identified authorization gaps.

Gate: three-account browser/API tests prove member and restricted-content behavior, upload ownership, bilateral blocking, group membership, expired/viewed snap behavior and private direct media URLs. Existing password/API-key fixtures still work. No Node backend removal before parity is demonstrated.

### 4. Alethea integration and evidence quality

Add native post text/media input to Alethea, service authentication, social identity/ownership mapping and versioned structured results. The social API enforces permissions before submission and every result/progress read. Users explicitly choose external processing and sharing; restricted content never becomes public to enable scraping.

Use durable jobs, bounded concurrency, per-user/platform budgets, request deduplication, timeouts and bounded retries. Store provider operation IDs/checkpoints where possible. If an interrupted external call cannot be reconciled, expose its uncertain state rather than promising exactly-once billing.

Improve retrieval from snippets to fetched relevant passages, linked to claim-level citations and retrieval dates. Preserve original source identity, distinguish repeated reporting from independent corroboration, and expose opposing evidence/uncertainty. Validate cited passages and source IDs; do not display uncalibrated model confidence as a truth probability. Treat source/model output as untrusted input and enforce safe outbound fetch rules.

Retain provider choice; never silently send a restricted post to another provider if the selected one fails. Keep inexpensive explanation separate from deeper research. Cache only with compatible content version, visibility, provider policy and freshness.

Gate: native text/image checks work; unauthorized users cannot retrieve results; mixed-claim and insufficient-evidence cases display correctly; jobs recover across restart; fact-checker outage does not affect posting. Use deterministic fixtures for integration and a small labeled evaluation set plus a budgeted live smoke test for evidence quality. Multiple agents agreeing is not by itself validation.

### 5. VPS release rehearsal

Finish production secret generation, migration startup, health checks, log rotation, process/resource limits, upload limits, backup/restore and upgrade instructions. Pin dependencies/images and preserve required project and dependency notices. Bind only Caddy HTTP/HTTPS ports publicly; keep databases, media, worker and Alethea internal.

Gate: clean Linux-host rehearsal from documented commands; three-account end-to-end flow; persistent data after recreation; encrypted backup restored to a clean instance with accounts/posts/media intact. Document schema-aware rollback and pre-upgrade backups. Run UI/type checks, Go tests/race checks where appropriate, authorization/integration tests and browser tests in CI.

Actual VPS publishing requires the host, SSH access and a domain/DNS configuration; these are not yet supplied. Repository packaging can be completed before those details exist. Automatic public HTTPS normally needs a domain pointing at the VPS and reachable validation ports.

## Deployment experience to deliver

Target operator flow (commands/scripts will be implemented, not currently available):

1. Install Docker Engine with Compose on the Linux VPS; point a domain to it.
2. Clone the repository and run a bootstrap command that generates unique secrets and asks for the domain.
3. Start the production Compose stack; migrations run as a controlled one-shot service.
4. Run the admin command to create the owner, then generate invitations.
5. Configure AI/search credentials if desired; configure off-host backup destination and recovery key storage.

Budgeting assumption: start by measuring a 2-vCPU/4-GB-RAM class machine for the pilot, with strict AI-job concurrency limits and external inference. This is a provisional engineering target, not a capacity guarantee or purchase recommendation. Storage sizing depends on uploads/retention. Prefer prebuilt images if builds stress the small host. No Kubernetes or separate managed infrastructure is necessary for this initial target.

## Agent execution plan

The orchestrator owns architecture, OpenAPI/schema decisions, authentication/authorization review, integration and final verification. Use at most three focused workers with non-overlapping file ownership; changes to shared contracts go through the orchestrator.

- Lower-cost, fresh-context agents handle clear component work, route scaffolding against a fixed contract, Docker packaging and documentation. Use explicit task briefs rather than forking the entire chat.
- Keep ambiguous backend behavior, migrations, auth and evidence correctness with the orchestrator or a suitably capable reviewer.
- Frontend owner edits web components/theme files; backend owner edits assigned Go modules; deployment owner edits container/runbook files. Shared generated files have one owner.
- Each assignment includes allowed files, relevant contract excerpts, acceptance checks and a short return format: changed files, checks run, unresolved risks.
- Integrate in small slices. Reuse agents only when their context remains useful. Avoid nested delegation and duplicated broad repository reviews.
- Make atomic commits for completed, checked changes. Only the orchestrator stages/commits; push the working branch at coherent checkpoints without force-pushing or bundling unrelated edits.
- Parallel execution is primarily a latency benefit. Smaller models may reduce cost per task; additional agents can increase total tokens. Do not promise a fixed saving.

The three initial planning reviews used inherited models. Future bounded implementation assignments should explicitly select a lighter model when appropriate.

## Current state and remaining inputs

The Go health/readiness service, checksum-verified SQL baseline, isolated container foundation and dark/light interactive frontend preview are implemented. See `FOUNDATION.md` for exact scope and commands. Go unit/race/integration checks, container build/startup and browser checks have passed locally. CI checks are defined separately. The preview is excluded from normal production bundles and uses fictional, local-only data.

Invitation accounts, Go social endpoints, private media integration, live Alethea checking, complete API contracts/generated clients, backups and the VPS release rehearsal remain. The original Node accounts still use Argon2 with open registration and JWT logout without server-side revocation; they are not the planned private-pilot authentication system. Docker was started for the local foundation rehearsal; no VPS has been deployed.

Confirmed: invitation-only, standard account security, one small Linux VPS. Provider/domain and off-host backup destination can be supplied at deployment time. Existing data preservation is the default even if the first pilot starts empty.

References: [Codex subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents), [Compose in production](https://docs.docker.com/compose/how-tos/production/), [Caddy automatic HTTPS](https://caddyserver.com/docs/automatic-https).
