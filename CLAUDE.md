# CLAUDE.md

## Project Overview

**Cryo** (`github.com/go-cryo/cryo`) — Kubernetes backup controller for managing PVC, PSQL, and S3 backups using restic. Provides a web UI for managing repository hosts (S3, SFTP, REST, local), repositories, backup jobs, and browsing snapshots.

## Build and Development Commands

### Go Backend
```bash
go build -o controller ./cmd/controller          # Build controller binary
go run ./cmd/controller                           # Run controller locally
go test ./...                                     # Run all tests
go mod tidy                                       # Clean up dependencies
go generate ./...                                 # Run code generation
just air                                          # Run with live reload
```

### UI (Quasar/Vue 3)
```bash
cd ui && npm install                              # Install dependencies
cd ui && npm run dev                              # Start dev server (port 9000)
cd ui && npm run build                            # Build for production
```

### Development Environment
```bash
just start                                        # Create KIND cluster with CSI driver and snapshot support
just stop                                         # Delete KIND cluster
just generate                                     # Run go generate
```

### Docker
```bash
docker build -f docker/controller.Dockerfile .    # Build controller image (multi-stage: Go 1.25.4 → Alpine 3.23.0)
docker build -f docker/pvc.Dockerfile .           # Build PVC backup image (Alpine with restic)
```

### Development Workflow

1. `just start` to create local KIND cluster with snapshot support
2. Terminal 1: `go run ./cmd/controller` (with `DEV=true` in `.env`)
3. Terminal 2: `cd ui && npm run dev`
4. UI at http://localhost:8080 (Go server proxies to Quasar dev server)

## Architecture

### Backend Packages (`internal/`)

| Package | Purpose |
|---------|---------|
| `server/` | Gin HTTP server — registers API routes, middleware, health/version endpoints. Includes BackupJobProvider, Scheduler, RunStore |
| `repositoryhost/` | Storage backend hosts (S3, SFTP, REST, local). Provider interface with Kubernetes Secrets storage |
| `repository/` | Restic repositories. References a host + path. Provider interface with Kubernetes Secrets storage |
| `restic/` | Restic CLI wrapper — `Check()` and `ListSnapshots()` via shell exec |
| `kubernetes/` | Kubernetes client, kubeconfig auto-detection, ConfigMap watch notifier with OnAdd/OnUpdate/OnDelete callbacks |
| `backupjob/` | BackupJob types, YAML config parsing, Provider interface, and Kubernetes ConfigMap-based storage |
| `scheduler/` | Cron scheduler using `robfig/cron/v3`. Maps backup jobs to cron entries, supports SyncAll/SyncJob/RemoveJob/TriggerNow |
| `executor/` | Creates K8s Jobs for PSQL/S3/PVC backups. Resolves repo credentials via repository.Provider. PVC orchestrator handles VolumeSnapshot lifecycle |
| `event/` | Event system with WebSocket support for real-time UI updates (10s ping interval). Includes EventObjectBackupJob and EventObjectBackupRun constants |
| `web/` | UI hosting — embeds static files in production, reverse proxies to Quasar dev server in dev mode |
| `util/` | Configuration (go-config), logging (zerolog), helper functions |

### Entry Point

`cmd/controller/main.go` — initializes config, logger, Kubernetes client, providers (RepositoryHost → Repository → Restic → BackupJob → Scheduler → Executor), and HTTP server.

### API Endpoints

Base URL: `/api/v1` (configurable via `API_BASE_URL`)

**Repository Hosts** (`server/host_controller.go`):
- `GET /hosts` — list all hosts
- `GET /hosts/:namespace/:name` — get host
- `POST /hosts` — create host
- `PUT /hosts/:namespace/:name` — update host
- `DELETE /hosts/:namespace/:name` — delete host (409 if referenced by repositories)

**Repositories** (`server/repository_controller.go`):
- `GET /repositories` — list all repositories
- `GET /repositories/:namespace/:name` — get repository
- `GET /repositories/:namespace/:name/check` — restic health check
- `GET /repositories/:namespace/:name/snapshots` — list restic snapshots
- `POST /repositories` — create repository
- `PUT /repositories/:namespace/:name` — update repository
- `DELETE /repositories/:namespace/:name` — delete repository

**Backup Jobs** (`server/backupjob_controller.go`):
- `GET /backupjobs` — list all backup jobs
- `GET /backupjobs/:namespace/:name` — get backup job (includes lastRun, nextRun)
- `POST /backupjobs` — create backup job
- `PUT /backupjobs/:namespace/:name` — update backup job
- `DELETE /backupjobs/:namespace/:name` — delete backup job
- `POST /backupjobs/:namespace/:name/trigger` — trigger backup job immediately (202)
- `GET /backupjobs/:namespace/:name/runs` — list backup runs

**Other**:
- `GET /health` — health check
- `GET /version` — service version
- `GET /ws` — WebSocket for real-time events

### Kubernetes Resources

All stored as **Kubernetes Secrets** with labels:
- Repository hosts: `go-cryo.github.com/repository-host=true`
  - Data: `BASE_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_DEFAULT_REGION`
- Repositories: `go-cryo.github.com/repository=true`
  - Data: `HOST_REF` (namespace/name), `PATH`, `RESTIC_PASSWORD`
  - Legacy format: `RESTIC_REPOSITORY` + credentials directly
- Backup configs (watched): `go-cryo.github.com/config=true` (ConfigMaps)

Repository resolution: `HOST_REF` → fetch host → build URL from `host.BaseURL + path` → merge credentials.

### Backup Job ConfigMap Format

Backup jobs are stored as **Kubernetes ConfigMaps** with label `go-cryo.github.com/config=true`, data key `config` containing YAML.

**Backup types**: `psql`, `s3`, `pvc`

**Common fields**: `type`, `schedule` (cron expression), `repositoryRef` (namespace/name), `suspend` (bool, optional), `image` (override default), `retention` (`keepLast`, `keepDaily`, `keepWeekly`, `keepMonthly`)

**Type-specific blocks**:
- `psql`: `hostname`, `port`, `username`, `database`, `passwordSecretRef`
- `s3`: `endpoint`, `bucket`, `credentialsSecretRef`
- `pvc`: `claimName`, `volumeSnapshotClassName`, `snapshotRetention`

### UI Structure (`ui/src/`)

Quasar 2 (Vue 3) with TypeScript. Axios base URL: `/api/v1`.

**Pages:**
| Route | Page | Purpose |
|-------|------|---------|
| `/` | IndexPage | Home/overview |
| `/hosts` | HostListPage | Host card grid |
| `/hosts/add` | HostAddPage | Create host form |
| `/hosts/:ns/:name` | HostDetailPage | Host details + edit/delete |
| `/hosts/:ns/:name/edit` | HostEditPage | Edit host form |
| `/repositories` | RepositoryListPage | Repository card grid |
| `/repositories/add` | RepositoryAddPage | Create repository form |
| `/repositories/:ns/:name` | RepositoryDetailPage | Repository details + snapshot table |
| `/repositories/:ns/:name/edit` | RepositoryEditPage | Edit repository form |
| `/backupjobs` | BackupJobListPage | Backup job card grid |
| `/backupjobs/add` | BackupJobAddPage | Create backup job form |
| `/backupjobs/:ns/:name` | BackupJobDetailPage | Job details + runs + trigger |
| `/backupjobs/:ns/:name/edit` | BackupJobEditPage | Edit backup job form |

**API clients** (`ui/src/api/`): `hosts.ts`, `repositories.ts`, `backupjobs.ts`, `version.ts`

**Layout**: `MainLayout.vue` — sidebar drawer with navigation (Home, Hosts, Repositories, Backup Jobs)

## Configuration

Environment variables via `github.com/mxcd/go-config` (`internal/util/config.go`):

| Variable | Default | Description |
|----------|---------|-------------|
| `DEPLOYMENT_IMAGE_TAG` | `development` | Docker image tag |
| `LOG_LEVEL` | `info` | Logging level (trace/debug/info/warn/error) |
| `ACCESS_LOGS` | `` | Comma-separated: `api`, `ui` |
| `PORT` | `8080` | HTTP server port |
| `API_BASE_URL` | `/api/v1` | API route prefix |
| `HEALTH_ENDPOINT` | `/health` | Health check path |
| `DEV` | `false` | Enables CORS, console logging, UI proxy |
| `STATIC_HOSTING` | `true` | Serve embedded UI files (false = proxy mode) |
| `UI_PROXY_URL` | `http://localhost:9000` | Quasar dev server URL for proxy mode |
| `KUBECONFIG_PATH` | `` | Path to kubeconfig (auto-detects ~/.kube/config or in-cluster) |
| `TARGET_NAMESPACE` | `` | Kubernetes namespace to watch |
| `PSQL_BACKUP_IMAGE` | `ghcr.io/go-cryo/cryo-psql:latest` | Docker image for PSQL backup jobs |
| `S3_BACKUP_IMAGE` | `ghcr.io/go-cryo/cryo-s3:latest` | Docker image for S3 backup jobs |
| `PVC_BACKUP_IMAGE` | `ghcr.io/go-cryo/cryo-pvc:latest` | Docker image for PVC backup jobs |

## Key Dependencies

**Go**: gin, gin-contrib/cors, gorilla/websocket, zerolog, go-config, k8s.io/client-go, gocron, robfig/cron/v3
**UI**: Vue 3, Quasar 2, Axios, TypeScript, Vue Router
