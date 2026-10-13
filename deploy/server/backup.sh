#!/usr/bin/env bash
# Dumps the birga database into the given folder and deletes dumps older than 14 days.
# Installed as /usr/local/bin/birga-backup; cron runs it nightly as the postgres user.
set -euo pipefail
dir="${1:-/var/backups/birga}"
pg_dump -Fc birga >"$dir/birga-$(date +%F).dump"
find "$dir" -name 'birga-*.dump' -mtime +14 -delete
