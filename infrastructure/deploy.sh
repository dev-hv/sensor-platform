#!/bin/bash
# -----------------------------------------------------------------------------
# Telemetry Platform — Deployment Orchestrator
#
# Builds and starts the full stack using Docker Compose. Intended for both
# manual operator runs on a local workstation and automated invocation via
# systemd on EC2 production nodes.
#
# Environment configuration is resolved automatically:
#   • Local:  uses infrastructure/.env when present.
#   • EC2:    fetches secrets from AWS SSM Parameter Store when .env is absent.
#
# TLS provisioning (EC2 only):
#   • Waits for DNS to propagate before requesting a Let's Encrypt certificate.
#   • Runs Certbot standalone on first boot, then mounts certs into Nginx.
# -----------------------------------------------------------------------------

set -euo pipefail

# TLS / domain configuration (production EC2)
DOMAIN="telemetry.vemurilabs.com"
CERT_EMAIL="hkvemuri@outlook.com"
CERT_LIVE_DIR="/etc/letsencrypt/live/${DOMAIN}"
DNS_POLL_INTERVAL_SEC=30
DNS_POLL_MAX_WAIT_SEC=600

# Resolve repository and infrastructure paths relative to this script.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
COMPOSE_BASE="${SCRIPT_DIR}/docker-compose.yml"
COMPOSE_PROD="${SCRIPT_DIR}/docker-compose.prod.yml"

SSM_READ_KEY_PATH="/telemetry/prod/READ_API_KEY"
SSM_WRITE_KEY_PATH="/telemetry/prod/WRITE_API_KEY"

LOCAL_DEPLOY=false

fetch_ssm_secret() {
  local param_name="$1"
  local result

  if ! result="$(aws ssm get-parameter \
    --name "${param_name}" \
    --with-decryption \
    --query "Parameter.Value" \
    --output text 2>&1)"; then
    echo "${result}" >&2
    return 1
  fi

  if [[ -z "${result}" || "${result}" == "None" ]]; then
    echo "Parameter ${param_name} returned an empty value." >&2
    return 1
  fi

  printf '%s' "${result}"
}

get_instance_public_ip() {
  local token
  token="$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" \
    -H "X-aws-ec2-metadata-token-ttl-seconds: 60" 2>/dev/null)" || return 1
  curl -sf -H "X-aws-ec2-metadata-token: ${token}" \
    "http://169.254.169.254/latest/meta-data/public-ipv4" 2>/dev/null
}

resolve_domain_ip() {
  if command -v dig >/dev/null 2>&1; then
    dig +short "${DOMAIN}" A 2>/dev/null | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | head -1
  elif command -v getent >/dev/null 2>&1; then
    getent ahosts "${DOMAIN}" 2>/dev/null | awk '/RAW/ && $1 ~ /^[0-9]+\./ { print $1; exit }'
  else
    python3 -c "import socket; print(socket.gethostbyname('${DOMAIN}'))" 2>/dev/null
  fi
}

wait_for_dns_propagation() {
  local instance_ip domain_ip elapsed=0

  echo "Waiting for DNS: ${DOMAIN} must resolve to this instance's public IP..."
  instance_ip="$(get_instance_public_ip)" || {
    echo "CRITICAL: Unable to determine EC2 public IP via instance metadata." >&2
    exit 1
  }
  echo "Instance public IP: ${instance_ip}"

  while [[ "${elapsed}" -lt "${DNS_POLL_MAX_WAIT_SEC}" ]]; do
    domain_ip="$(resolve_domain_ip || true)"
    if [[ -n "${domain_ip}" && "${domain_ip}" == "${instance_ip}" ]]; then
      echo "DNS propagation confirmed: ${DOMAIN} -> ${domain_ip}"
      return 0
    fi

    echo "DNS not ready (${DOMAIN} -> ${domain_ip:-unresolved}, expected ${instance_ip}). Retrying in ${DNS_POLL_INTERVAL_SEC}s..."
    sleep "${DNS_POLL_INTERVAL_SEC}"
    elapsed=$((elapsed + DNS_POLL_INTERVAL_SEC))
  done

  echo "CRITICAL: DNS for ${DOMAIN} did not point to ${instance_ip} within ${DNS_POLL_MAX_WAIT_SEC} seconds." >&2
  echo "          Update your Namecheap A record, then re-run deployment." >&2
  exit 1
}

stop_containers_on_port_80() {
  echo "Stopping Docker containers that may be bound to port 80..."

  docker compose \
    --env-file "${ENV_FILE}" \
    -f "${COMPOSE_BASE}" \
    -f "${COMPOSE_PROD}" \
    down 2>/dev/null || true

  local cid
  for cid in $(docker ps -q 2>/dev/null); do
    if docker port "${cid}" 80 2>/dev/null | grep -q .; then
      docker stop "${cid}" >/dev/null 2>&1 || true
    fi
  done
}

run_certbot() {
  local -a certbot_cmd

  if [[ "$(id -u)" -eq 0 ]]; then
    certbot_cmd=(certbot)
  else
    certbot_cmd=(sudo certbot)
  fi

  "${certbot_cmd[@]}" certonly \
    --standalone \
    --non-interactive \
    --agree-tos \
    -m "${CERT_EMAIL}" \
    -d "${DOMAIN}"
}

provision_initial_ssl() {
  if [[ -d "${CERT_LIVE_DIR}" ]]; then
    echo "TLS certificate already present at ${CERT_LIVE_DIR}. Skipping initial provisioning."
    return 0
  fi

  if ! command -v certbot >/dev/null 2>&1; then
    echo "CRITICAL: Certbot is not installed. Cannot provision TLS certificate." >&2
    exit 1
  fi

  echo "No TLS certificate found for ${DOMAIN}. Beginning initial Let's Encrypt provisioning..."

  wait_for_dns_propagation
  stop_containers_on_port_80

  echo "Requesting certificate from Let's Encrypt (standalone mode)..."
  if ! run_certbot; then
    echo "CRITICAL: Certbot failed to obtain a certificate for ${DOMAIN}." >&2
    echo "          Verify DNS, port 80 reachability, and Let's Encrypt rate limits." >&2
    exit 1
  fi

  if [[ ! -d "${CERT_LIVE_DIR}" ]]; then
    echo "CRITICAL: Certbot reported success but ${CERT_LIVE_DIR} was not created." >&2
    exit 1
  fi

  echo "TLS certificate successfully provisioned for ${DOMAIN}."
}

# -----------------------------------------------------------------------------
# Environment resolution — local .env file or AWS SSM Parameter Store
# -----------------------------------------------------------------------------
if [[ -f "${ENV_FILE}" ]]; then
  LOCAL_DEPLOY=true
  echo "Local .env file detected. Proceeding with deployment."
else
  echo "No .env file found. Attempting to fetch keys from AWS SSM Parameter Store..."

  if ! command -v aws >/dev/null 2>&1; then
    echo "CRITICAL: AWS CLI is not installed. Cannot retrieve secrets from SSM." >&2
    exit 1
  fi

  SSM_ERROR=""
  if ! READ_API_KEY="$(fetch_ssm_secret "${SSM_READ_KEY_PATH}")"; then
    SSM_ERROR="Failed to retrieve ${SSM_READ_KEY_PATH}."
  elif ! WRITE_API_KEY="$(fetch_ssm_secret "${SSM_WRITE_KEY_PATH}")"; then
    SSM_ERROR="Failed to retrieve ${SSM_WRITE_KEY_PATH}."
  fi

  if [[ -n "${SSM_ERROR}" ]]; then
    echo "CRITICAL: Failed to retrieve API keys from AWS SSM Parameter Store." >&2
    echo "          Verify the instance IAM role grants ssm:GetParameter on:" >&2
    echo "            ${SSM_READ_KEY_PATH}" >&2
    echo "            ${SSM_WRITE_KEY_PATH}" >&2
    echo "          Detail: ${SSM_ERROR}" >&2
    exit 1
  fi

  cat > "${ENV_FILE}" <<EOF
# API keys: Read for GET, Write for POST
# Auto-generated by deploy.sh from AWS SSM Parameter Store
READ_API_KEY=${READ_API_KEY}
WRITE_API_KEY=${WRITE_API_KEY}
EOF

  chmod 600 "${ENV_FILE}"
  echo "Successfully generated ${ENV_FILE} from SSM parameters."
fi

# -----------------------------------------------------------------------------
# Pre-flight checks, TLS provisioning (production), and stack deployment
# -----------------------------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: Docker is not installed or not on PATH." >&2
  exit 1
fi

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-sensor-platform}"

if [[ "${LOCAL_DEPLOY}" == false ]]; then
  provision_initial_ssl
fi

echo "Deploying telemetry stack from ${SCRIPT_DIR}..."

docker compose \
  --env-file "${ENV_FILE}" \
  -f "${COMPOSE_BASE}" \
  -f "${COMPOSE_PROD}" \
  up -d --build --remove-orphans

echo "Deployment complete. Verify containers with: docker compose -f ${COMPOSE_BASE} ps"
