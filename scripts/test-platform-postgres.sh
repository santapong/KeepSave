#!/usr/bin/env bash
# Synthetic databases only. Never read or forward an operator DATABASE_URL.
set -euo pipefail
project_root=$(cd "$(dirname "$0")/.." && pwd)
run_dir=$(mktemp -d "${TMPDIR:-/tmp}/keepsave-platform-tests.XXXXXXXX")
run_id=${run_dir##*/}
network=$run_id
database_container="${run_id}-postgres"
test_container="${run_id}-go"
network_created=0
database_created=0
test_created=0
cleanup() {
  status=$?
  trap - EXIT
  if [ "$test_created" = 1 ]; then docker rm -f "$test_container" >/dev/null 2>&1 || true; fi
  if [ "$database_created" = 1 ]; then docker rm -f "$database_container" >/dev/null 2>&1 || true; fi
  if [ "$network_created" = 1 ]; then docker network rm "$network" >/dev/null 2>&1 || true; fi
  rmdir "$run_dir"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker network create --label "keepsave.test-run=$run_id" "$network" >/dev/null
network_created=1
docker create --name "$database_container" --label "keepsave.test-run=$run_id" \
  --network "$network" --network-alias keepsave-platform-postgres-test \
  --network-alias keepsave-social-postgres-test --tmpfs /var/lib/postgresql/data \
  -e POSTGRES_USER=keepsave_platform_test -e POSTGRES_PASSWORD=local-test-only \
  -e POSTGRES_DB=keepsave_platform_test \
  postgres:16@sha256:1a6ab3f5345eb6dbe04a1349529caabdb0ab09293a09590fad07b2246bfa4b54 >/dev/null
database_created=1
docker start "$database_container" >/dev/null

# The image's temporary initialization server listens only on a Unix socket.
# Require TCP readiness so migrations cannot race that server's shutdown.
ready=0
for attempt in $(seq 1 60); do
  if docker exec "$database_container" pg_isready -h 127.0.0.1 \
    -U keepsave_platform_test -d keepsave_platform_test -t 1 >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [ "$ready" != 1 ]; then
  docker logs "$database_container"
  echo "Disposable PostgreSQL did not become ready" >&2
  exit 1
fi

# The social fixture has its own synthetic database and a fixed alias on this
# run's network. The identity and platform fixtures isolate themselves by schema.
docker exec "$database_container" psql -U keepsave_platform_test -d keepsave_platform_test \
  -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public" \
  -c "CREATE ROLE keepsave_social_test LOGIN PASSWORD 'local-test-only'" \
  -c "CREATE DATABASE keepsave_social_test OWNER keepsave_social_test" >/dev/null
docker create --name "$test_container" --label "keepsave.test-run=$run_id" \
  --cpus=2 --network "$network" -e GOMAXPROCS=2 \
  -e KEEPSAVE_PLATFORM_POSTGRES_TEST=1 -e KEEPSAVE_SOCIAL_POSTGRES_TEST=1 \
  -v "$project_root/backend:/src" -w /src \
  golang:1.27.1@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190 \
  go test -p 2 -race -count=1 ./internal/api ./internal/service ./internal/repository ./cmd/keepsave-vault \
    -run '^(TestPlatform|TestIdentity|TestSocialAuthPostgres)' -timeout 300s >/dev/null
test_created=1
docker start -a "$test_container"
# docker start reports daemon errors; inspect the actual test process exit code.
test_status=$(docker inspect --format '{{.State.ExitCode}}' "$test_container")
exit "$test_status"
