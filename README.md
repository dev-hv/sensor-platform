# Sensor Platform

![Architect](https://img.shields.io/badge/Architect-Hari%20Krishna%20Vemuri-1e3a8a?style=flat-square&logo=account&logoColor=white)
![AI-Assisted](https://img.shields.io/badge/Development-AI--Assisted-2563EB?style=flat-square&logo=sparkles&logoColor=white)

A secure, automated MedTech telemetry platform for ingesting, persisting, and visualizing vital stats from edge devices. The stack combines a schema-driven Go API, Postgres persistence, a React dashboard, and fully automated AWS EC2 deployment with Let's Encrypt TLS.

## Architecture Flow

```
 Edge Devices / Simulator                AWS EC2 (Production)
 ┌─────────────────────┐                ┌──────────────────────────────────────────┐
 │  simulator.py       │   HTTPS POST   │  Nginx (TLS :443)                        │
 │  (mock telemetry)   │ ──────────────►│    └─► /api/* → Go Ingestion API (:8080) │
 └─────────────────────┘                │              │                           │
                                        │              ▼                           │
                                        │         Postgres (:5432)                   │
                                        │              │                           │
                                        │              ▼                           │
                                        │         React Dashboard (static)         │
                                        └──────────────────────────────────────────┘
```

**Data path**

1. **Sensor Simulator** — `simulator.py` generates JSON payloads (`timestamp`, `serial_number`, `temperature`, `humidity`) and POSTs to `/ingest` with the write API key.
2. **Nginx TLS Reverse Proxy** — Terminates HTTPS, redirects HTTP→HTTPS, proxies `/api/` to the Go backend on the host loopback.
3. **Go Ingestion API** — Validates payloads against `infrastructure/schema.yaml`, enforces min/max bounds, INSERTs into Postgres.
4. **Postgres Persistence** — Stores time-series rows in `sensor_metrics`; table shape is derived from the YAML schema at startup.
5. **React Dashboard** — Polls `/schema` and `/metrics` with the read API key; renders live charts.

## Repository Layout

| Path | Description |
|------|-------------|
| `backend/` | Go HTTP API — ingest, query, schema endpoints |
| `frontend/` | React + Vite dashboard, Nginx configs |
| `infrastructure/` | Docker Compose, schema, bootstrap, deploy scripts |
| `simulator.py` | Python telemetry simulation harness (repo root) |

See each folder's `README.md` for layer-specific design notes.

## Prerequisites

- **Local development:** Docker Desktop, Node.js 20+, Go 1.25+, Python 3.10+
- **Production (EC2):** Ubuntu 22.04+ AMI, IAM instance profile with SSM access
- **DNS:** Namecheap A record for your telemetry domain pointing to the EC2 public IP

## Usage & Deployment

### 1. Production — EC2 Bootstrap (User Data)

Paste `infrastructure/bootstrap.sh` into the EC2 **User Data** field when launching the instance. On first boot it will:

1. Install Docker, Docker Compose v2, AWS CLI, and Certbot
2. Clone the repository using a GitHub token from SSM (`/telemetry/prod/GITHUB_TOKEN`)
3. Register and start `telemetry-boot.service`, which invokes `deploy.sh`

**Required SSM parameters**

| Parameter | Purpose |
|-----------|---------|
| `/telemetry/prod/GITHUB_TOKEN` | Private repo clone |
| `/telemetry/prod/READ_API_KEY` | Dashboard / read API |
| `/telemetry/prod/WRITE_API_KEY` | Device ingest |

**Security group:** allow inbound TCP **80** and **443**.

Before the first deployment completes TLS provisioning, create a Namecheap **A record** for your telemetry domain (e.g. `telemetry.vemurilabs.com`) pointing to the EC2 instance public IP. `deploy.sh` polls DNS for up to 10 minutes on first boot before requesting a Let's Encrypt certificate.

### 2. Production — Manual Deploy

SSH into the EC2 host (or re-run after config changes):

```bash
cd /home/ubuntu/sensor-platform
./infrastructure/deploy.sh
```

`deploy.sh` automatically:

- Fetches API keys from SSM when `infrastructure/.env` is absent
- Waits for DNS propagation before requesting a Let's Encrypt certificate (first boot)
- Builds and starts the Docker Compose stack

### 3. Local Development

Create `infrastructure/.env` with your API keys:

```env
READ_API_KEY=your-read-key
WRITE_API_KEY=your-write-key
```

Start the stack (local compose, no TLS):

```bash
docker compose --env-file infrastructure/.env \
  -f infrastructure/docker-compose.yml \
  up -d --build
```

Open the dashboard at [http://localhost:3000](http://localhost:3000).

### 4. Telemetry Simulation

Install Python dependencies:

```bash
pip install -r requirements.txt
```

Run against the local backend (default):

```bash
python simulator.py
```

Run against production:

```bash
python simulator.py prod
```

The simulator reads `WRITE_API_KEY` from `infrastructure/.env` and POSTs mock device data every 15 seconds.

## Schema

Telemetry columns are defined in `infrastructure/schema.yaml`. The backend and database DDL are generated from this file at runtime—no hard-coded metric fields in application code.

Current dynamic metrics: **temperature**, **humidity**.

## License

Proprietary — Vemuri Labs.

### 🏛️ Architecture & Provenance

* **Design & Architecture:** All system design and security decisions by Hari Krishna Vemuri.
* **Implementation:** Application code, infrastructure-as-code (IaC), and deployment scripts written via Generative AI assistance based on the architectural blueprint.
