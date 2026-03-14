#!/usr/bin/env bash
set -euo pipefail

# KeepSave Docker Compose Verification Script
# Builds, starts, health-checks, and tears down the full stack.

COMPOSE_FILE="${1:-docker-compose.yml}"
PROJECT="keepsave-verify"
TIMEOUT=120  # seconds to wait for healthy services

cleanup() {
  echo ">>> Tearing down..."
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

echo "=== KeepSave Docker Compose Verification ==="
echo ""

# Step 1: Validate compose file
echo ">>> Step 1: Validating compose file..."
docker compose -f "$COMPOSE_FILE" config --quiet
echo "    Compose file is valid."

# Step 2: Build images
echo ">>> Step 2: Building images..."
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" build --no-cache
echo "    Build succeeded."

# Step 3: Start services
echo ">>> Step 3: Starting services..."
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" up -d
echo "    Services started."

# Step 4: Wait for all services to be healthy
echo ">>> Step 4: Waiting for services to become healthy (timeout: ${TIMEOUT}s)..."

elapsed=0
interval=5
while [ $elapsed -lt $TIMEOUT ]; do
  all_healthy=true

  for svc in db api frontend; do
    status=$(docker compose -p "$PROJECT" -f "$COMPOSE_FILE" ps --format json "$svc" 2>/dev/null \
      | jq -r '.Health // .State' 2>/dev/null || echo "unknown")

    if [ "$status" != "healthy" ] && [ "$status" != "running" ]; then
      all_healthy=false
    fi
  done

  if $all_healthy; then
    echo "    All services are up after ${elapsed}s."
    break
  fi

  sleep $interval
  elapsed=$((elapsed + interval))
done

if [ $elapsed -ge $TIMEOUT ]; then
  echo "!!! TIMEOUT: Services did not become healthy within ${TIMEOUT}s."
  echo ""
  echo "--- Container status ---"
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" ps
  echo ""
  echo "--- API logs ---"
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" logs api --tail 50
  echo ""
  echo "--- DB logs ---"
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" logs db --tail 30
  exit 1
fi

# Step 5: Smoke tests
echo ">>> Step 5: Running smoke tests..."

# Test DB connectivity via API health endpoint
echo -n "    API /health/ready ... "
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health/ready || true)
if [ "$HTTP_CODE" = "200" ]; then
  echo "OK (200)"
else
  echo "FAIL (HTTP $HTTP_CODE)"
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" logs api --tail 20
  exit 1
fi

# Test frontend is serving
echo -n "    Frontend :3000 ... "
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/ || true)
if [ "$HTTP_CODE" = "200" ]; then
  echo "OK (200)"
else
  echo "FAIL (HTTP $HTTP_CODE)"
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" logs frontend --tail 20
  exit 1
fi

echo ""
echo "=== Verification PASSED ==="
