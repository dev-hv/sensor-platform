# Backend

## Purpose

The Go backend is the authoritative ingestion and query API for edge-device telemetry. It validates payloads against `infrastructure/schema.yaml`, persists rows to Postgres, and exposes authenticated read endpoints for the React dashboard.

## Components

| Path | Role |
|------|------|
| `main.go` | Entry point: loads schema, initializes Postgres, registers HTTP routes, starts the server. |
| `internal/schema/` | YAML schema structs (`Schema`, `ColumnDef`, `DynamicCol`). |
| `internal/db/init.go` | Connects to Postgres and creates the metrics table from schema (idempotent `CREATE TABLE IF NOT EXISTS`). |
| `internal/handlers/` | `ingest_handler.go` (POST validation + INSERT), `metrics_handler.go` (time-range queries), `schema_handler.go` (schema JSON for the UI). |
| `internal/middleware/` | API-key authentication (`X-API-Key`) and CORS. |
| `internal/config/secrets.go` | Resolves `READ_API_KEY` / `WRITE_API_KEY` from environment or AWS SSM on EC2. |
| `Dockerfile` | Multi-stage build producing a minimal Alpine runtime image. |

## Design Philosophy

**Schema-driven, not hard-coded.** Dynamic columns (e.g., `temperature`, `humidity`) are defined only in `schema.yaml`. The ingest handler iterates `DynamicColumns` for validation bounds and INSERT column lists—adding or removing a metric requires a schema change, not a Go code change.

**Fail closed on auth.** Every route passes through `middleware.Auth`. Read and write keys are distinct; ingestion requires the write key, dashboard queries require the read key.

**Container-friendly configuration.** `SCHEMA_PATH`, `DB_*`, and `PORT` are environment-driven. In Docker, the schema file is bind-mounted to `/schema.yaml`; locally, the default path is `../infrastructure/schema.yaml`.

**No ORM, minimal surface area.** Raw SQL with parameterized queries keeps the data path auditable for MedTech-style traceability. Table creation is idempotent for greenfield deploys; existing Postgres volumes retain prior columns until manually migrated.

## API Surface

| Method | Path | Key | Description |
|--------|------|-----|-------------|
| `POST` | `/ingest` | Write | Accept JSON telemetry payload. |
| `GET` | `/metrics` | Read | Query time-series data with optional device and window filters. |
| `GET` | `/schema` | Read | Return active schema for dashboard rendering. |
