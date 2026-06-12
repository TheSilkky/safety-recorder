#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: compose/smoke-test.sh [full|sqlite-local|postgresql-local|sqlite-s3|relay-sqlite-local] [-- <simclient args>]

Runs a Docker Compose smoke stack, waits for the private admin dashboard, then runs
the Go simulator against the containerized server.

Environment:
  PROOFLINE_MAIN_PORT     Host port for the main API/viewer. Default: 18080
  PROOFLINE_ADMIN_PORT    Host port for private-admin routes. Default: 18081
  PROOFLINE_RELAY_PORT    Host port for the stream-ingress relay smoke variant. Default: 18090
  PROOFLINE_PRIVATE_PORT  Legacy alias for PROOFLINE_MAIN_PORT.
  PROOFLINE_PUBLIC_PORT   Legacy alias for PROOFLINE_ADMIN_PORT.
  PROOFLINE_SMOKE_BOOTSTRAP_SECRET  Local bootstrap secret for the container.
  PROOFLINE_SMOKE_USERNAME          Local account username. Default: admin
  PROOFLINE_SMOKE_PASSWORD          Local account password.
  PROOFLINE_SMOKE_RELAY_SERVICE_TOKEN      Local relay-to-core service token.
  PROOFLINE_SMOKE_RELAY_CAPABILITY_SECRET  Local core relay capability secret.
  PROOFLINE_SMOKE_SECRETS_DIR       Secret-file directory mounted into TOML stacks.
  COMPOSE_PROJECT_NAME    Compose project name. Default: proofline-smoke-<variant>
  KEEP_COMPOSE=1          Leave containers and volumes running after the test.
USAGE
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

variant="full"
if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi
if [[ $# -gt 0 && "$1" != "--" ]]; then
  variant="$1"
  shift
fi
if [[ $# -gt 0 && "$1" == "--" ]]; then
  shift
fi

case "$variant" in
  full)
    compose_file="$script_dir/compose-full.yml"
    ;;
  sqlite-local)
    compose_file="$script_dir/compose-sqlite-local.yml"
    ;;
  postgresql-local)
    compose_file="$script_dir/compose-postgresql-local.yml"
    ;;
  sqlite-s3)
    compose_file="$script_dir/compose-sqlite-s3.yml"
    ;;
  relay-sqlite-local)
    compose_file="$script_dir/compose-relay-sqlite-local.yml"
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

if docker compose version >/dev/null 2>&1; then
  compose=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
  compose=(docker-compose)
else
  echo "docker compose or docker-compose is required" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required to wait for the containerized private admin dashboard" >&2
  exit 1
fi

export PROOFLINE_MAIN_PORT="${PROOFLINE_MAIN_PORT:-${PROOFLINE_PRIVATE_PORT:-18080}}"
export PROOFLINE_ADMIN_PORT="${PROOFLINE_ADMIN_PORT:-${PROOFLINE_PUBLIC_PORT:-18081}}"
export PROOFLINE_RELAY_PORT="${PROOFLINE_RELAY_PORT:-18090}"
export PROOFLINE_SMOKE_BOOTSTRAP_SECRET="${PROOFLINE_SMOKE_BOOTSTRAP_SECRET:-replace-with-local-compose-bootstrap-secret}"
export PROOFLINE_SMOKE_USERNAME="${PROOFLINE_SMOKE_USERNAME:-admin}"
export PROOFLINE_SMOKE_PASSWORD="${PROOFLINE_SMOKE_PASSWORD:-replace-with-a-long-local-password}"
export PROOFLINE_SMOKE_RELAY_SERVICE_TOKEN="${PROOFLINE_SMOKE_RELAY_SERVICE_TOKEN:-replace-with-local-relay-service-token}"
export PROOFLINE_SMOKE_RELAY_CAPABILITY_SECRET="${PROOFLINE_SMOKE_RELAY_CAPABILITY_SECRET:-replace-with-local-relay-capability-secret}"
export PROOFLINE_SMOKE_POSTGRES_DSN="${PROOFLINE_SMOKE_POSTGRES_DSN:-postgres://proofline:proofline@postgres:5432/proofline?sslmode=disable}"
export PROOFLINE_SMOKE_S3_ACCESS_KEY_ID="${PROOFLINE_SMOKE_S3_ACCESS_KEY_ID:-proofline}"
export PROOFLINE_SMOKE_S3_SECRET_ACCESS_KEY="${PROOFLINE_SMOKE_S3_SECRET_ACCESS_KEY:-proofline-minio-password}"
export PROOFLINE_SMOKE_VALKEY_PASSWORD="${PROOFLINE_SMOKE_VALKEY_PASSWORD:-proofline-valkey-password}"
project="${COMPOSE_PROJECT_NAME:-proofline-smoke-${variant}}"
default_runtime_secrets_dir="$script_dir/.smoke-secrets/$project"
export PROOFLINE_SMOKE_SECRETS_DIR="${PROOFLINE_SMOKE_SECRETS_DIR:-$default_runtime_secrets_dir}"
sim_args=("$@")

cleanup() {
  status=$?
  if [[ "${KEEP_COMPOSE:-0}" == "1" ]]; then
    echo "Leaving compose stack running: project=$project file=$compose_file"
    exit "$status"
  fi
  "${compose[@]}" -p "$project" -f "$compose_file" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$default_runtime_secrets_dir" 2>/dev/null || true
  exit "$status"
}
trap cleanup EXIT

prepare_smoke_secrets() {
  mkdir -p "$PROOFLINE_SMOKE_SECRETS_DIR"
  chmod 755 "$PROOFLINE_SMOKE_SECRETS_DIR"
  printf '%s\n' "$PROOFLINE_SMOKE_BOOTSTRAP_SECRET" >"$PROOFLINE_SMOKE_SECRETS_DIR/auth-bootstrap-secret.example"
  printf '%s\n' "$PROOFLINE_SMOKE_POSTGRES_DSN" >"$PROOFLINE_SMOKE_SECRETS_DIR/postgres-dsn.example"
  printf '%s\n' "$PROOFLINE_SMOKE_S3_ACCESS_KEY_ID" >"$PROOFLINE_SMOKE_SECRETS_DIR/s3-access-key-id.example"
  printf '%s\n' "$PROOFLINE_SMOKE_S3_SECRET_ACCESS_KEY" >"$PROOFLINE_SMOKE_SECRETS_DIR/s3-secret-access-key.example"
  printf '%s\n' "$PROOFLINE_SMOKE_VALKEY_PASSWORD" >"$PROOFLINE_SMOKE_SECRETS_DIR/valkey-password.example"
  chmod 644 "$PROOFLINE_SMOKE_SECRETS_DIR"/*.example
}

wait_for_admin_dashboard() {
  local url="http://127.0.0.1:${PROOFLINE_ADMIN_PORT}/admin/static/styles.css"
  for _ in $(seq 1 60); do
    if curl --fail --silent --output /dev/null "$url"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_relay_readiness() {
  local live_url="http://127.0.0.1:${PROOFLINE_RELAY_PORT}/health/live"
  local ready_url="http://127.0.0.1:${PROOFLINE_RELAY_PORT}/health/ready"
  for _ in $(seq 1 60); do
    if curl --fail --silent --output /dev/null "$live_url" &&
      curl --fail --silent --output /dev/null "$ready_url"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

assert_relay_does_not_mount_server_routes() {
  local route
  local status
  for route in /admin /admin/api/accounts /v1/incidents /i/viewer-token /metrics; do
    status="$(curl --silent --show-error --output /dev/null --write-out "%{http_code}" "http://127.0.0.1:${PROOFLINE_RELAY_PORT}${route}")"
    if [[ "$status" != "404" ]]; then
      echo "relay route ${route} returned HTTP ${status}, want 404" >&2
      return 1
    fi
  done
}

bootstrap_admin() {
  local url="http://127.0.0.1:${PROOFLINE_ADMIN_PORT}/admin/bootstrap"
  local response_file
  local status

  response_file="$(mktemp)"

  status="$(curl --silent --show-error --output "$response_file" --write-out "%{http_code}" \
    -X POST "$url" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode "bootstrap_secret=${PROOFLINE_SMOKE_BOOTSTRAP_SECRET}" \
    --data-urlencode "username=${PROOFLINE_SMOKE_USERNAME}" \
    --data-urlencode "password=${PROOFLINE_SMOKE_PASSWORD}")"

  case "$status" in
    303)
      rm -f "$response_file"
      return 0
      ;;
    *)
      echo "admin bootstrap failed with HTTP ${status}" >&2
      sed -n '1,20p' "$response_file" >&2 || true
      rm -f "$response_file"
      return 1
      ;;
  esac
}

cd "$repo_root"

prepare_smoke_secrets
"${compose[@]}" -p "$project" -f "$compose_file" down -v --remove-orphans >/dev/null 2>&1 || true
if ! "${compose[@]}" -p "$project" -f "$compose_file" up --build -d; then
  "${compose[@]}" -p "$project" -f "$compose_file" ps || true
  "${compose[@]}" -p "$project" -f "$compose_file" logs --no-color || true
  exit 1
fi

if ! wait_for_admin_dashboard; then
  "${compose[@]}" -p "$project" -f "$compose_file" ps
  "${compose[@]}" -p "$project" -f "$compose_file" logs --no-color
  echo "server did not serve the admin dashboard on private-admin port ${PROOFLINE_ADMIN_PORT}" >&2
  exit 1
fi

if ! bootstrap_admin; then
  "${compose[@]}" -p "$project" -f "$compose_file" ps
  "${compose[@]}" -p "$project" -f "$compose_file" logs --no-color
  exit 1
fi

if [[ "$variant" == "relay-sqlite-local" ]]; then
  if ! wait_for_relay_readiness; then
    "${compose[@]}" -p "$project" -f "$compose_file" ps
    "${compose[@]}" -p "$project" -f "$compose_file" logs --no-color
    echo "stream-ingress relay did not become ready on relay port ${PROOFLINE_RELAY_PORT}" >&2
    exit 1
  fi
  if ! assert_relay_does_not_mount_server_routes; then
    "${compose[@]}" -p "$project" -f "$compose_file" ps
    "${compose[@]}" -p "$project" -f "$compose_file" logs --no-color
    exit 1
  fi
  echo "relay smoke passed: core admin listener and stream-ingress readiness are available"
  exit 0
fi

PROOFLINE_SIM_USERNAME="$PROOFLINE_SMOKE_USERNAME" \
PROOFLINE_SIM_PASSWORD="$PROOFLINE_SMOKE_PASSWORD" \
go run ./cmd/simclient \
  --api "http://127.0.0.1:${PROOFLINE_MAIN_PORT}" \
  --viewer "http://127.0.0.1:${PROOFLINE_MAIN_PORT}" \
  --setup-totp-second-factor \
  --chunks 3 \
  --interval 0s \
  --download-bundle \
  "${sim_args[@]}" \
  | sed -E 's#(Incident viewer: https?://[^[:space:]]+/(i|e)/)[^[:space:]]+#\1[redacted]#'
