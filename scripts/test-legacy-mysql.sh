#!/usr/bin/env bash
# Fixed disposable MySQL target only. No operator DSN is read or accepted.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/.." && pwd)
run_id="$(date +%s)-$$-$RANDOM"
network="keepsave-mysql-tests-$run_id"
container="keepsave-backend-mysql-test-$run_id"
created=0
network_created=0
cleanup(){
 if [ "$created" = 1 ] && [ "$(docker inspect --format '{{index .Config.Labels "keepsave.legacy-mysql-verification"}}' "$container")" = core-20261002 ]; then
  docker rm -f "$container" >/dev/null
 fi
 if [ "$network_created" = 1 ] && [ "$(docker network inspect --format '{{index .Labels "keepsave.legacy-mysql-verification"}}' "$network")" = core-20261002 ]; then
  docker network rm "$network" >/dev/null
 fi
}
trap cleanup EXIT
docker network create --label keepsave.legacy-mysql-verification=core-20261002 "$network" >/dev/null
network_created=1
docker run -d --name "$container" --network "$network" --label keepsave.legacy-mysql-verification=core-20261002 \
 --network-alias keepsave-backend-mysql-test-20261002 --tmpfs /var/lib/mysql \
 -e MYSQL_ROOT_PASSWORD=local-test-only -e MYSQL_ROOT_HOST=% mysql:8.4@sha256:6ea90827b1100f8f2ae306a539f86d2c264a26ed435a2a9f75551dd5c3aeb242 >/dev/null
created=1
ready=0
for attempt in $(seq 1 60);do
 if docker exec -e MYSQL_PWD=local-test-only "$container" mysqladmin ping -h 127.0.0.1 -u root --silent >/dev/null 2>&1;then ready=1;break;fi
 sleep 1
done
if [ "$ready" != 1 ];then echo 'Disposable MySQL did not become ready' >&2;exit 1;fi
# Cache volumes contain compiler/dependency artifacts, never application data.
docker run --rm --cpus=2 --memory=2g --network "$network" --label keepsave.legacy-mysql-verification=core-20261002 -e GOMAXPROCS=2 -e CGO_ENABLED=1 -e KEEPSAVE_MYSQL_TEST=1 \
 -v keepsave-go-mod:/go/pkg/mod -v keepsave-go-build:/root/.cache/go-build \
 -v "$repo_root/backend:/src" -w /src golang:1.27.1@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190 \
 go test -race -p 2 -count=1 ./internal/api -run '^TestPlatformMySQLLegacy' -timeout 180s
