#!/bin/sh
# Generate local secrets without displaying them. Existing config is never overwritten.
set -eu
cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
umask 077

if [ -e .env.pilot ] || [ -e .secrets ]; then
  echo 'Existing .env.pilot or .secrets found; refusing to overwrite.' >&2
  exit 1
fi

site=${1:---local}
case "$site" in
  --local)
    address=http://localhost
    bind=127.0.0.1
    http_port=18080
    https_port=18443
    preview=true
    ;;
  *)
    # Accept a DNS hostname only; prevent Caddy/env configuration injection.
    case "$site" in
      *[!a-zA-Z0-9.-]*|.*|*..*|*.)
        echo 'Pass --local or a DNS hostname such as social.example.com.' >&2
        exit 1
        ;;
    esac
    case "$site" in
      *.*) ;;
      *) echo 'A public DNS hostname is required.' >&2; exit 1 ;;
    esac
    address=$site
    bind=0.0.0.0
    http_port=80
    https_port=443
    preview=false
    ;;
esac

command -v openssl >/dev/null
mkdir .secrets
password=$(openssl rand -hex 32)
printf '%s' "$password" > .secrets/database_password
printf 'postgres://tassm:%s@postgres:5432/tassm?sslmode=disable' "$password" > .secrets/database_url
unset password
# The parent directory remains 0700 on the host. Mounted secret files must
# be readable by the non-root application UID inside its container.
chmod 644 .secrets/database_password .secrets/database_url
cat > .env.pilot <<EOF
SITE_ADDRESS=$address
BIND_ADDRESS=$bind
HTTP_PORT=$http_port
HTTPS_PORT=$https_port
ENABLE_PREVIEW=$preview
EOF
echo 'Created private configuration. This is a foundation preview, not the finished social pilot.'
echo 'Start: docker compose --env-file .env.pilot -f compose.pilot.yaml up -d --build'
