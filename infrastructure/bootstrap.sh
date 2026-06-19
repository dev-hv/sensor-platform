#!/bin/bash
# -----------------------------------------------------------------------------
# AWS EC2 User Data — Telemetry Platform Bootstrap
#
# Provisions a bare-metal Ubuntu instance with container runtime, TLS tooling,
# and a systemd unit that invokes deploy.sh on boot. Environment configuration
# (local .env or AWS SSM secrets) is handled entirely by deploy.sh.
#
# Intended usage: paste or reference this script as EC2 "User Data" when
# launching the instance. Cloud-init executes it as root on initial boot.
# -----------------------------------------------------------------------------

set -euxo pipefail

export DEBIAN_FRONTEND=noninteractive

REPO_DIR="/home/ubuntu/sensor-platform"
DEPLOY_SCRIPT="${REPO_DIR}/infrastructure/deploy.sh"
SYSTEMD_UNIT="/etc/systemd/system/telemetry-boot.service"
CERTBOT_HOOK_DIR="/etc/letsencrypt/renewal-hooks/post"
CERTBOT_HOOK_SCRIPT="${CERTBOT_HOOK_DIR}/reload-nginx.sh"

# -----------------------------------------------------------------------------
# Section 1: Base package index refresh
# -----------------------------------------------------------------------------
apt-get update -y
apt-get upgrade -y

# -----------------------------------------------------------------------------
# Section 2: Container runtime — Docker Engine and Compose v2 plugin
# -----------------------------------------------------------------------------
apt-get install -y \
  ca-certificates \
  curl \
  gnupg \
  lsb-release

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "${VERSION_CODENAME}") stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

usermod -aG docker ubuntu

# -----------------------------------------------------------------------------
# Section 3: AWS CLI — required by deploy.sh for SSM secret retrieval
# -----------------------------------------------------------------------------
apt-get install -y awscli

# -----------------------------------------------------------------------------
# Section 4: Certbot — automated TLS certificate lifecycle management
# -----------------------------------------------------------------------------
apt-get install -y certbot

# -----------------------------------------------------------------------------
# Section 5: Application repository — secure clone from GitHub
# -----------------------------------------------------------------------------
mkdir -p "${REPO_DIR}"
chown -R ubuntu:ubuntu "${REPO_DIR}"

echo "Fetching GitHub Access Token from AWS SSM..."
GITHUB_TOKEN="$(aws ssm get-parameter \
  --name "/telemetry/prod/GITHUB_TOKEN" \
  --with-decryption \
  --query "Parameter.Value" \
  --output text)"

if [[ -z "${GITHUB_TOKEN}" || "${GITHUB_TOKEN}" == "None" ]]; then
  echo "ERROR: Failed to retrieve GitHub token. Cannot clone repository." >&2
  exit 1
fi

echo "Cloning secure repository..."
sudo -u ubuntu git clone "https://oauth2:${GITHUB_TOKEN}@github.com/dev-hv/sensor-platform.git" "${REPO_DIR}"

# -----------------------------------------------------------------------------
# Section 6: Certbot post-renewal hook — reload frontend reverse proxy
# -----------------------------------------------------------------------------
mkdir -p "${CERTBOT_HOOK_DIR}"

cat > "${CERTBOT_HOOK_SCRIPT}" <<'HOOK_EOF'
#!/bin/bash
# Reload nginx inside the telemetry_frontend container after certificate renewal.
set -euo pipefail
if docker ps --format '{{.Names}}' | grep -qx 'telemetry_frontend'; then
  docker exec telemetry_frontend nginx -s reload
fi
HOOK_EOF

chmod 0755 "${CERTBOT_HOOK_SCRIPT}"

# -----------------------------------------------------------------------------
# Section 7: Systemd oneshot — invoke deploy.sh on boot (env handled there)
# -----------------------------------------------------------------------------
cat > "${SYSTEMD_UNIT}" <<UNIT_EOF
[Unit]
Description=Telemetry Platform — Docker Compose deployment
After=docker.service network-online.target
Wants=network-online.target
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
User=ubuntu
Group=ubuntu
WorkingDirectory=${REPO_DIR}/infrastructure
ExecStart=${DEPLOY_SCRIPT}
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
UNIT_EOF

systemctl daemon-reload
systemctl enable telemetry-boot.service

# -----------------------------------------------------------------------------
# Section 8: Trigger initial deployment
# -----------------------------------------------------------------------------
# deploy.sh resolves API keys from infrastructure/.env or AWS SSM at runtime.
systemctl start telemetry-boot.service
