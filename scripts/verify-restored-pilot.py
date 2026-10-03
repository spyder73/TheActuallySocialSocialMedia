#!/usr/bin/env python3
"""Compare deterministic hashes of every public table in live and restored DBs.

Read-only. Run before switching databases or revoking restored sessions.
Uses the Compose project selected by COMPOSE_PROJECT_NAME (default tassm-pilot).
"""
import os
import re
import subprocess
import sys

compose = ['docker', 'compose', '--env-file', '.env.pilot', '-f', 'compose.pilot.yaml', 'exec', '-T', 'postgres', 'psql', '-U', 'tassm', '-At', '-v', 'ON_ERROR_STOP=1']


def query(database, sql):
    return subprocess.check_output(compose + ['-d', database, '-c', sql], text=True).strip()


if len(sys.argv) != 2 or not re.fullmatch(r'tassm_restore_[a-z0-9_]+', sys.argv[1]):
    raise SystemExit('Usage: scripts/verify-restored-pilot.py tassm_restore_DATABASE')
source, restored = os.environ.get('BACKUP_DATABASE', 'tassm'), sys.argv[1]
listing = "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename"
tables = query(source, listing)
assert tables == query(restored, listing), 'Restored table inventory differs'
for table in tables.splitlines():
    quoted = '"' + table.replace('"', '""') + '"'
    # Hashes are compared in memory, never dumped with account data to logs.
    sql = f"SELECT count(*)::text || ':' || coalesce(md5(string_agg(row_hash, '' ORDER BY row_hash)), '') FROM (SELECT md5(row_to_json(t)::text) row_hash FROM {quoted} t) rows"
    assert query(source, sql) == query(restored, sql), f'Restored data differs in {table}'
print(f'Restore verified: all rows in {len(tables.splitlines())} tables match.')
