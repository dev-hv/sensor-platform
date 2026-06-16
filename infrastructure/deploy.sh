#!/bin/bash
# -----------------------------------------------------------------------------
# Telemetry Platform — Deployment Orchestrator
#
# Builds and starts the full stack using Docker Compose. Intended for both
# manual operator runs and automated invocation via systemd on EC2 boot.
# -----------------------------------------------------------------------------

set -euo pipefail

# Resolve repository and infrastructure paths relative to this script.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${REPO_ROOT}/.env"
COMPOSE_BASE="${SCRIPT_DIR}/docker-compose.yml"
COMPOSE_PROD="${SCRIPT_DIR}/docker-compose.prod.yml"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "ERROR: Environment file not found at ${ENV_FILE}" >&2
  echo "       Provision READ_API_KEY, WRITE_API_KEY, and database credentials before deploying." >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: Docker is not installed or not on PATH." >&2
  exit 1
fi

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-sensor-platform}"

echo "Deploying telemetry stack from ${SCRIPT_DIR}..."

docker compose \
  --env-file "${ENV_FILE}" \
  -f "${COMPOSE_BASE}" \
  -f "${COMPOSE_PROD}" \
  up -d --build --remove-orphans

echo "Deployment complete. Verify containers with: docker compose -f ${COMPOSE_BASE} ps"
