#!/usr/bin/env bash
# Restore into a NEW database only; never overwrite the live database.
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077
if [[ $# != 3 ]]; then
  echo 'Usage: scripts/restore-pilot.sh BACKUP.age IDENTITY_FILE NEW_DATABASE' >&2; exit 2
fi
command -v age >/dev/null
backup=$1
identity=$2
database=$3
[[ "$database" =~ ^tassm_restore_[a-z0-9_]+$ ]] || {
  echo 'Database must start with tassm_restore_ and contain only lowercase letters, digits, underscores.' >&2; exit 2;
}
[[ ${#database} -le 63 ]] || { echo 'Database name too long.' >&2; exit 2; }
compose=(docker compose --env-file .env.pilot -f compose.pilot.yaml)
# Authentication is checked before creating a database. No plaintext file is written.
age -d -i "$identity" "$backup" > /dev/null
"${compose[@]}" exec -T postgres createdb -U tassm "$database"
if ! age -d -i "$identity" "$backup" | "${compose[@]}" exec -T postgres \
  pg_restore -U tassm -d "$database" --no-owner --no-acl --exit-on-error --single-transaction; then
  echo "Restore failed; live database is untouched. Inspect/remove the empty database $database before retrying." >&2
  exit 1
fi
echo "Restored into $database. Live application has NOT been switched. Follow docs/DEPLOY.md to validate and activate."
