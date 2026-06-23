# Infrastructure

## Purpose

This directory owns all Infrastructure-as-Code (IaC) for the telemetry platform: container orchestration, schema-driven database shape, EC2 bootstrap automation, and production deployment. Application source code lives in sibling folders (`backend/`, `frontend/`); this layer wires those services together in local and AWS environments.

## Components

| File | Role |
|------|------|
| `docker-compose.yml` | Base stack: Postgres, Go backend, React frontend (local development). |
| `docker-compose.prod.yml` | Production override: host-networked backend, TLS-terminated Nginx, Let's Encrypt volume mounts. |
| `schema.yaml` | Single source of truth for mandatory and dynamic telemetry columns; drives Postgres DDL and API validation. |
| `deploy.sh` | Environment-aware deployment orchestrator: resolves secrets from `infrastructure/.env` (local) or AWS SSM (EC2), provisions initial TLS via Certbot, runs Docker Compose. |
| `bootstrap.sh` | EC2 User Data script: installs Docker, AWS CLI, Certbot; clones the repo; registers a systemd unit that invokes `deploy.sh` on boot. |
| `.env` | Local-only API keys and database credentials (git-ignored). Not used on EC2 production nodes. |

## Design Philosophy

**Separation of concerns.** Keeping IaC out of application folders makes it clear what is deployable artifact versus what is runtime wiring. Compose build contexts reference `../backend` and `../frontend` so images always track the latest application code.

**Environment-aware secrets.** Local developers commit nothing sensitive: a checked-in `.env.example` pattern with a real `.env` on disk. Production nodes have no static secrets; `deploy.sh` pulls `/telemetry/prod/*` parameters from SSM at runtime and writes a `chmod 600` env file.

**Zero-touch TLS.** First-boot production deploy waits for DNS propagation (Namecheap A record → EC2 public IP) before calling Let's Encrypt standalone, avoiding rate-limit failures. Certificates are mounted read-only into Nginx; Certbot renewal hooks reload the frontend container.

**Systemd process boundary.** `telemetry-boot.service` runs as the `ubuntu` user with a fixed `WorkingDirectory`, delegating all configuration logic to `deploy.sh`. Bootstrap installs dependencies only; it does not generate environment files or request certificates.

**Least privilege (IAM).** The EC2 instance role should grant only `ssm:GetParameter` on required paths (`READ_API_KEY`, `WRITE_API_KEY`, `GITHUB_TOKEN`) and standard CloudWatch/logging permissions—never blanket `ssm:*` or `s3:*`.
