#!/usr/bin/env bash
# Encrypt a consistent PostgreSQL snapshot; the private age identity stays off-server.
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077
if [[ $# != 2 ]]; then
  echo 'Usage: scripts/backup-pilot.sh AGE_RECIPIENT OUTPUT.age' >&2; exit 2
fi
command -v age >/dev/null
recipient=$1
output=$2
[[ ! -e "$output" ]] || { echo 'Refusing to overwrite backup.' >&2; exit 1; }
mkdir -p "$(dirname "$output")"
temporary=$(mktemp "${output}.partial.XXXXXX")
trap 'rm -f "$temporary"' EXIT
docker compose --env-file .env.pilot -f compose.pilot.yaml exec -T postgres \
  pg_dump -U tassm -d "${BACKUP_DATABASE:-tassm}" --format=custom --no-owner --no-acl \
  | age -r "$recipient" > "$temporary"
[[ -s "$temporary" ]]
mv "$temporary" "$output"
echo "Encrypted database backup written to $output"
echo 'Copy it off-host. Keep your age identity and deployment secrets separately.'
