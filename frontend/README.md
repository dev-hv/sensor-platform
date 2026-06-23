# Frontend

## Purpose

The React dashboard is the operator-facing visualization layer. It polls the backend for schema metadata and live metrics, rendering multi-series charts with time-window presets. In production, all traffic is served over HTTPS via the Nginx container defined in `infrastructure/docker-compose.prod.yml`.

## Components

| Path | Role |
|------|------|
| `src/App.jsx` | Main dashboard: schema fetch, metrics polling, Recharts line chart, time-range controls. |
| `src/main.jsx` | React entry point. |
| `vite.config.js` | Dev-server proxy to the backend (`/api` → `localhost:8080`). |
| `Dockerfile` | Builds static assets with `VITE_READ_API_KEY` baked in at compile time; serves via Nginx. |
| `nginx.conf` | Production Nginx: HTTP→HTTPS redirect, TLS termination, `/api/` reverse proxy to host-bound backend. |
| `nginx.local.conf` | Local development Nginx config (no TLS). |

## Design Philosophy

**Build-time read key, runtime proxy.** Because React runs in the browser, the read API key is embedded via Vite (`import.meta.env.VITE_READ_API_KEY`) during `docker build`. The write key never touches the frontend.

**Same-origin API in production.** Nginx terminates TLS and proxies `/api/*` to the Go backend on the host loopback, so the browser never calls the backend directly and CORS stays simple.

**Schema-driven UI.** Chart series and Y-axis assignment are derived from `GET /schema`; removing a metric from `schema.yaml` automatically removes it from the dashboard without frontend code changes.

**Containerized static delivery.** Production ships a pre-built `dist/` bundle inside an Nginx Alpine image. No Node.js runtime on the EC2 host—only the compiled assets and Nginx worker processes.
