#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
work_dir=$(mktemp -d)
data_dir=$(mktemp -d)
socket_dir=$(mktemp -d)
port=${TASMANE_TEST_POSTGRES_PORT:-55439}

cleanup() {
  if [[ -x "$work_dir/root/usr/lib/postgresql/14/bin/pg_ctl" && -f "$data_dir/PG_VERSION" ]]; then
    LD_LIBRARY_PATH="$work_dir/root/usr/lib/x86_64-linux-gnu" \
      "$work_dir/root/usr/lib/postgresql/14/bin/pg_ctl" -D "$data_dir" -m immediate stop >/dev/null 2>&1 || true
  fi
  rm -rf -- "$work_dir" "$data_dir" "$socket_dir"
}
trap cleanup EXIT

cd "$work_dir"
apt-get download postgresql-14 postgresql-client-14 postgresql-common libpq5 >/dev/null
for package in ./*.deb; do
  dpkg-deb -x "$package" root
done

pg_bin="$work_dir/root/usr/lib/postgresql/14/bin"
export LD_LIBRARY_PATH="$work_dir/root/usr/lib/x86_64-linux-gnu"
"$pg_bin/initdb" -D "$data_dir" -A trust -U postgres --no-locale >/dev/null
"$pg_bin/pg_ctl" -D "$data_dir" -l "$work_dir/postgres.log" \
  -o "-k $socket_dir -p $port -h 127.0.0.1" -w start >/dev/null

export TEST_DATABASE_URL="postgres://postgres@127.0.0.1:$port/postgres?sslmode=disable"
export PATH="/home/ichiken/.local/toolchains/go1.22.12/bin:$PATH"
export GOFLAGS=-buildvcs=false
cd "$repo_dir"
go test -count=1 -run TestMigrationAppliesToFreshPostgresAndEnforcesSchema -v ./db
