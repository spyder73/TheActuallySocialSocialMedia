# Go/container foundation checkpoint

This checkpoint runs the new Go health/readiness API and static frontend behind Caddy, with an isolated PostgreSQL database. It does **not** yet provide Go social routes, invitation accounts, Alethea, private media, or the completed VPS pilot. Keep using the original development stack for existing social functionality.

## Local rehearsal

Requires Docker Engine/Desktop with Compose and OpenSSL.

```sh
sh scripts/bootstrap-pilot.sh --local
docker compose --env-file .env.pilot -f compose.pilot.yaml up -d --build
curl --fail http://localhost:18080/api/healthz
curl --fail http://localhost:18080/api/readyz
```

Open `/preview` on port 18080 for the clearly labeled, local-only interactive design proof. The bootstrap enables it only in local mode. Preview content is fictional and not saved to the database. Ordinary frontend login/social routes still require the original Node API and are not supported by this foundation.

This stack uses its own Compose project/volumes and binds only loopback ports locally. It does not reuse or reset the original development database. `.env.pilot` and `.secrets/` are ignored by Git and excluded from Docker build contexts. Compose file secrets are mounted files, not an encrypted secret vault: protect the host and retain restrictive file permissions.

The Go migration command applies an embedded copy of the existing SQL baseline. See `apps/api-go/README.md` for ledger compatibility and failure behavior. Always back up before pointing a migration runner at an existing database. The host secret directory is mode 0700; its files are readable by the non-root API after Docker mounts them inside that container.

## Future VPS configuration

On a fresh checkout, `sh scripts/bootstrap-pilot.sh social.example.com` writes the public hostname and ports 80/443, with preview disabled. Point DNS to the VPS first. Caddy persists certificates and automatically manages HTTPS. Database ports are never published.

This is packaging groundwork; do not invite users until the authentication, authorization, media, backup/restore and release gates in `REBUILD_PLAN.md` pass. No real VPS was deployed by this checkpoint.

## Commands

```sh
# Inspect startup without printing secret contents
docker compose --env-file .env.pilot -f compose.pilot.yaml ps
docker compose --env-file .env.pilot -f compose.pilot.yaml logs --tail=50 api migrate

# Stop containers, retaining data
docker compose --env-file .env.pilot -f compose.pilot.yaml down
```

Do not use `down --volumes` on a deployment with data you need. Named volumes are persistence, not backups. The bootstrap refuses to overwrite existing configuration so that rerunning it cannot silently rotate database credentials.
