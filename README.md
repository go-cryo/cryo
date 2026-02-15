<p align="center">
  <img src="docs/img/icon.png" alt="Cryo" width="200" />
</p>

<h1 align="center">Cryo</h1>

<p align="center">
  Kubernetes backup controller for PSQL, S3, and PVC workloads using <a href="https://restic.net">restic</a>.
</p>

<p align="center">
  <a href="https://github.com/go-cryo/cryo/actions"><img src="https://github.com/go-cryo/cryo/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/go-cryo/cryo/releases"><img src="https://img.shields.io/github/v/release/go-cryo/cryo" alt="Release" /></a>
  <a href="https://github.com/go-cryo/cryo/blob/main/LICENSE"><img src="https://img.shields.io/github/license/go-cryo/cryo" alt="License" /></a>
</p>

---

Cryo runs inside your Kubernetes cluster and provides a web UI and API for managing backup jobs. It supports three backup types:

- **PostgreSQL** -- dumps databases via `pg_dump` and stores them in a restic repository
- **S3** -- mirrors S3-compatible buckets into a restic repository
- **PVC** -- creates VolumeSnapshots of PersistentVolumeClaims, mounts them as temporary volumes, and backs up the files into a restic repository

Backups are scheduled via cron expressions and stored in restic repositories backed by S3, SFTP, REST, or local storage.

## Features

- Cron-based backup scheduling with manual trigger support
- Repository host abstraction (S3, SFTP, REST, local) with credential management
- VolumeSnapshot-based PVC backups with configurable snapshot retention
- Restic repository health checks and snapshot browsing
- Real-time status updates via WebSocket
- Web UI built with Vue 3 and Quasar
- Configurable retention policies (keep last, daily, weekly, monthly)

## Quick Start

### Prerequisites

- Kubernetes cluster (1.27+)
- Helm 3
- For PVC backups: a CSI driver with snapshot support and a VolumeSnapshotClass

### Deploy with Docker

```bash
docker pull ghcr.io/go-cryo/cryo:latest
```

Container images are published for `linux/amd64` and `linux/arm64`:

| Image | Purpose |
|-------|---------|
| `ghcr.io/go-cryo/cryo` | Controller with web UI |
| `ghcr.io/go-cryo/cryo-psql` | PostgreSQL backup worker |
| `ghcr.io/go-cryo/cryo-s3` | S3 backup worker |
| `ghcr.io/go-cryo/cryo-pvc` | PVC backup worker |

## Configuration

Cryo is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | Logging level (trace/debug/info/warn/error) |
| `DEV` | `false` | Enable dev mode (CORS, console logging, UI proxy) |
| `TARGET_NAMESPACE` | | Kubernetes namespace to watch |
| `PSQL_BACKUP_IMAGE` | `ghcr.io/go-cryo/cryo-psql:latest` | PostgreSQL backup worker image |
| `S3_BACKUP_IMAGE` | `ghcr.io/go-cryo/cryo-s3:latest` | S3 backup worker image |
| `PVC_BACKUP_IMAGE` | `ghcr.io/go-cryo/cryo-pvc:latest` | PVC backup worker image |

## Development

### Prerequisites

- Go 1.25+
- Node.js 18+
- Docker
- [just](https://github.com/casey/just) command runner
- [KIND](https://kind.sigs.k8s.io/) for local Kubernetes

### Getting Started

```bash
# Create a local KIND cluster with CSI snapshot support
just start

# Run the backend (Terminal 1)
go run ./cmd/controller

# Run the UI dev server (Terminal 2)
cd ui && npm install && npm run dev
```

The UI is available at `http://localhost:8080` (the Go server proxies to the Quasar dev server in dev mode).

### Running Tests

```bash
just test          # Unit tests with race detection
just test-e2e      # E2E tests against KIND cluster
just test-all      # Both
```

### Building

```bash
go build -o controller ./cmd/controller

# Docker images
docker build -f docker/controller.Dockerfile .
docker build -f docker/psql.Dockerfile hack/backup
docker build -f docker/s3.Dockerfile hack/backup
docker build -f docker/pvc.Dockerfile hack/backup
```

## Architecture

Cryo stores all configuration as native Kubernetes resources:

- **Repository Hosts** -- Kubernetes Secrets holding storage backend credentials
- **Repositories** -- Kubernetes Secrets referencing a host + restic password
- **Backup Jobs** -- ConfigMaps with YAML configuration (type, schedule, retention, source)
- **Settings** -- ConfigMap for global settings (job TTL, default storage class)

The controller watches these resources, schedules backup jobs via cron, and creates Kubernetes Jobs for each backup execution. Backup workers run as short-lived pods that perform the actual restic operations.

## API

Base URL: `/api/v1`

| Endpoint | Description |
|----------|-------------|
| `GET/POST /hosts` | List/create repository hosts |
| `GET/PUT/DELETE /hosts/:ns/:name` | Manage a host |
| `GET/POST /repositories` | List/create repositories |
| `GET /repositories/:ns/:name/snapshots` | Browse restic snapshots |
| `GET/POST /backupjobs` | List/create backup jobs |
| `POST /backupjobs/:ns/:name/trigger` | Trigger a backup immediately |
| `GET /backupjobs/:ns/:name/runs` | List backup runs |
| `GET /health` | Health check |
| `GET /ws` | WebSocket for real-time events |

## License

See [LICENSE](LICENSE) for details.
