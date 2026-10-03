# Deploy the invitation-only pilot

This guide targets one Linux VPS, a domain you control, and 2–10 invited members. The Go pilot is a text-first release: chronological posts, replies/comments, member profiles, follows, close-friend visibility, blocking, reports, and private/group text conversations. The original Node implementation is retained in the repository. Media, stories/snaps, AI conversations, and live Alethea checks have not been ported; do not migrate a populated legacy community expecting feature parity.

Accounts use Argon2id password hashes and revocable database sessions. HTTPS encrypts transport. Messages and posts are readable by the server; they are not end-to-end encrypted. No public registration, email delivery service, or default password is provided.

## 1. Prepare the host

Use a maintained Linux distribution. A 2-vCPU/4-GB-RAM host is an initial engineering target; measure your own usage, especially while building images. Install Git, OpenSSL, Docker Engine with the Compose plugin, and `age` from your distribution/vendor. Docker Desktop is not required on a VPS. Follow the [Docker Engine installation guide](https://docs.docker.com/engine/install/) for your distribution.

Point your domain's A record (and AAAA record only if IPv6 works) at the VPS. Allow inbound TCP 80/443; restrict SSH to your administrative access. Do not open PostgreSQL or API ports. Caddy obtains and renews certificates automatically; DNS and validation ports must be reachable. See [Caddy automatic HTTPS](https://caddyserver.com/docs/automatic-https).

Docker access is effectively root access. Use a dedicated operating account, apply OS updates, and protect the host and off-host backups.

## 2. Clone and configure

```sh
git clone --branch rebuild/go-pilot https://github.com/spyder73/TheActuallySocialSocialMedia.git
cd TheActuallySocialSocialMedia
sh scripts/bootstrap-pilot.sh social.example.com
```

Replace `social.example.com` with your real domain. Bootstrap generates random database credentials under `.secrets/` and writes `.env.pilot`; it refuses to overwrite existing configuration. Both paths are excluded from Git and Docker build contexts. Keep the secret directory mode `0700`; its files must remain readable inside containers by the non-root API UID. Compose file secrets protect distribution, but are not an encrypted vault.

`APP_ORIGIN` must exactly match the browser origin, including the HTTPS scheme, with no trailing slash. This is the allowed origin for account and social mutations. `ALLOW_LOCAL_HTTP=false` is required publicly. `ENABLE_PREVIEW=false` excludes fictional demo pages from production builds.

For a local rehearsal only:

```sh
sh scripts/bootstrap-pilot.sh --local
```

Local mode binds loopback port 18080 and uses `http://localhost:18080`. Use that exact hostname, not `127.0.0.1`; origin checks intentionally reject a mismatch. Do not expose local HTTP mode to the network.

## 3. Build and start

```sh
docker compose --env-file .env.pilot -f compose.pilot.yaml up -d --build --wait
docker compose --env-file .env.pilot -f compose.pilot.yaml ps
curl --fail https://social.example.com/api/readyz
```

A one-shot migration service runs before the API starts. SQL checksums are verified, and unknown/untracked existing schemas fail closed. A migration failure prevents the new API from starting. Never edit an applied SQL file to fix it; add a new migration. Adoption also rejects legacy accounts whose emails or usernames differ only by case; resolve those identities explicitly in a backed-up staging copy before cutover.

Only Caddy publishes host ports. PostgreSQL and API use an internal Docker network. Containers restart after host reboot when Docker itself is enabled. Named volumes retain the database and Caddy certificates.

## 4. Create the owner and invite members

The server admin CLI creates email-bound, expiring, one-use links; the recipient chooses their own password in the browser. Treat each link like a password until redeemed. Transfer it through your chosen private channel. Links use URL fragments so the token is not sent in normal HTTP request paths.

```sh
# Bootstrap exactly one administrator invitation; redeem it before proceeding.
docker compose --env-file .env.pilot -f compose.pilot.yaml exec api /app/admin invite --admin owner@example.com
# Invite each additional member.
docker compose --env-file .env.pilot -f compose.pilot.yaml exec api /app/admin invite friend@example.com
# Recovery and access management (USER can be exact ID, email or username).
docker compose --env-file .env.pilot -f compose.pilot.yaml exec api /app/admin reset USER
docker compose --env-file .env.pilot -f compose.pilot.yaml exec api /app/admin disable USER
docker compose --env-file .env.pilot -f compose.pilot.yaml exec api /app/admin sessions revoke USER
```

Invitations expire after seven days; reset links after one hour. Issuing another link for the same account invalidates the previous unused link. Passwords must be at least 12 characters. Save the generated link privately and open it in your browser. The email must match the invitation exactly (case is normalized).

Review reports and remove an offending post through the server CLI:

```sh
docker compose --env-file .env.pilot -f compose.pilot.yaml exec api /app/admin reports
docker compose --env-file .env.pilot -f compose.pilot.yaml exec api /app/admin remove-post POST_ID
```

Post removal is permanent and cascades to its comments/replies under the existing schema. Review the target carefully first.

No administrator password or invitation is seeded at startup. Standard accounts cannot create invitations through the web UI. Server administrators run the CLI.

## 5. Back up before inviting real users

Generate an age key on a trusted machine, not on the public VPS:

```sh
age-keygen -o tassm-backup-identity.txt
```

Keep this identity offline or in your password manager. Copy only its public `age1...` recipient to the VPS. Install `age` on the VPS, then:

```sh
scripts/backup-pilot.sh age1REPLACE_WITH_YOUR_PUBLIC_RECIPIENT /path/to/backups/tassm-2026-10-03.age
```

The script streams a transactionally consistent PostgreSQL custom-format dump directly into age encryption. It refuses to overwrite an existing output and removes partial output on failure. No unencrypted dump is written to disk. Copy the encrypted archive off-host after each run. Schedule daily backups using your host's scheduler and monitor the job's exit status. Keep several daily and weekly generations, subject to your privacy/retention policy.

Back up `.env.pilot` and `.secrets/` separately in encrypted storage, along with the Git commit used for the deployment. The DB backup includes password hashes, sessions, posts, and messages, so protect both archives and the private age identity. Caddy certificate volumes can be reissued; losing the DB cannot be repaired without a backup. Current pilot data is database-only; when media support is added, the backup procedure must include those objects too.

If you activate a restored database with a different name, set `BACKUP_DATABASE` to that name for subsequent backup jobs.

## 6. Rehearse restoration

Restoration creates a **new database** and never overwrites or switches the live one. Keep sufficient free disk space. Use a separate rehearsal host where possible.

```sh
scripts/restore-pilot.sh /path/to/backup.age /secure/path/tassm-backup-identity.txt tassm_restore_rehearsal
```

The script verifies decryption before creating the database and uses a single transaction for `pg_restore`. It refuses an existing database name. If import fails, the original application stays untouched; inspect the newly created empty database before removing it or retrying with another name.

On a quiet rehearsal instance, `python3 scripts/verify-restored-pilot.py tassm_restore_rehearsal` compares every table and row against the source database without printing contents. Stop writes first; otherwise legitimate changes after the backup will cause a mismatch.

Check restored row counts with `docker compose ... exec -T postgres psql -U tassm -d tassm_restore_rehearsal`, then test accounts and posting on a separate deployment. Do not treat an archive as a working backup until a restoration rehearsal succeeds.

To activate a restored database on this host during an announced maintenance window:

1. Stop the API: `docker compose --env-file .env.pilot -f compose.pilot.yaml stop api`.
2. Preserve the old `.secrets/database_url` in encrypted recovery storage.
3. Change **only the database name** in `.secrets/database_url` from `tassm` to the restored database name. Do not print the connection string into logs or commit it. Retain the original user, password, hostname, and `sslmode`.
4. Run `docker compose --env-file .env.pilot -f compose.pilot.yaml run --rm migrate` with the desired release to check/apply its schema migrations.
5. Revoke restored sessions before going live (they may include previously logged-out sessions at backup time); run `docker compose --env-file .env.pilot -f compose.pilot.yaml run --rm --entrypoint /app/admin api sessions revoke-all`. This also invalidates outstanding invite/reset links; issue new ones afterwards.
6. Recreate the API: `docker compose --env-file .env.pilot -f compose.pilot.yaml up -d --force-recreate api`.
7. Check readiness, sign-in, a restricted post, and a private conversation. Set `BACKUP_DATABASE` for all future backup jobs to the activated name.

Keep the previous database until the restoration has been verified. Never run `down --volumes` on a deployment whose data you need.

## 7. Upgrade and rollback

Record the currently deployed commit and take an encrypted backup first. Prefer testing the candidate commit and restoration on a separate instance. Do not use blind unattended `git pull` on a live service.

```sh
git fetch origin
git checkout REVIEWED_COMMIT
# Build while the current containers still serve traffic.
docker compose --env-file .env.pilot -f compose.pilot.yaml build
# Maintenance window: stop API before schema changes.
docker compose --env-file .env.pilot -f compose.pilot.yaml stop api
docker compose --env-file .env.pilot -f compose.pilot.yaml run --rm migrate
docker compose --env-file .env.pilot -f compose.pilot.yaml up -d --wait
```

Recheck readiness and sign-in afterwards. A code rollback is safe only if the old release supports the upgraded schema. Otherwise restore the pre-upgrade backup into a new database and deploy the matching old commit; this loses changes since that backup. The migration runner deliberately rejects a database containing migrations unknown to the older binary. No automatic destructive down-migration is provided.

## Operations and limits

```sh
docker compose --env-file .env.pilot -f compose.pilot.yaml logs --tail=100 api migrate web
docker compose --env-file .env.pilot -f compose.pilot.yaml stats --no-stream
docker compose --env-file .env.pilot -f compose.pilot.yaml down
```

The last command stops services while retaining volumes. Log rotation is configured. Monitor available disk, backup success, certificate renewal, and API readiness. There is no email-based password recovery: administrators issue reset links. Disabling an account revokes its sessions.

The pilot has bounded list sizes and password-hashing concurrency. It is intended for a small trusted group, not unrestricted public signup. Conversations refresh through HTTP; realtime SSE, uploads and the optional Alethea service are future migration milestones. No external AI provider receives pilot content in this release.

## Release verification

The repository's CI runs Go tests including database integration, frontend production builds, and Docker startup checks. See the release notes in `FOUNDATION.md` for the exact checks performed on this revision. A real VPS, public DNS/TLS issuance, and off-host backup scheduling must be verified on your own host; repository tests cannot prove those operational steps.
