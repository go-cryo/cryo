# Cryo Examples

Example Kubernetes manifests for configuring Cryo backup controller resources.

## Overview

Cryo uses native Kubernetes resources for all configuration:

| Resource | Kind | Purpose |
|----------|------|---------|
| Repository Host | `Secret` | Storage backend connection (S3, SFTP, REST, local) |
| Repository | `Secret` | Restic repository referencing a host |
| Backup Job | `ConfigMap` | Backup schedule, source, and retention policy |

## Directory Structure

```
examples/
  hosts/
    s3-host.yaml          # S3-compatible storage host
    sftp-host.yaml        # SFTP storage host
    rest-host.yaml        # REST server host
    local-host.yaml       # Local filesystem host
  repositories/
    repository.yaml       # Restic repository referencing a host
  backupjobs/
    psql-backup.yaml      # PostgreSQL database backup
    psql-credentials.yaml # PostgreSQL password secret
    s3-backup.yaml        # S3 bucket backup
    s3-credentials.yaml   # S3 access credentials secret
    pvc-backup.yaml       # PVC volume backup
```

## Concepts

### Repository Hosts

A repository host defines **where** restic repositories are stored. It is a Kubernetes Secret with the label `go-cryo.github.com/repository-host: "true"`.

The `BASE_URL` field determines the host type:

| Prefix | Type | Example |
|--------|------|---------|
| `s3:` | S3 | `s3:https://my-bucket.s3.eu-central-1.amazonaws.com` |
| `sftp:` | SFTP | `sftp:user@host:` |
| `rest:` | REST | `rest:https://backup.example.com` |
| *(none)* | Local | `/backups` |

S3 hosts require additional credential fields: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and optionally `AWS_DEFAULT_REGION`.

### Repositories

A repository represents a single restic repository. It is a Kubernetes Secret with the label `go-cryo.github.com/repository: "true"`.

Repositories reference a host via `HOST_REF` (format: `namespace/name`) and specify a `PATH` within that host. The final restic repository URL is constructed as `host.BASE_URL + "/" + PATH`.

Each repository has its own `RESTIC_PASSWORD` used for encryption.

### Backup Jobs

A backup job defines **what** to back up, **when**, and **where** to store it. It is a Kubernetes ConfigMap with the label `go-cryo.github.com/config: "true"` containing a YAML configuration under the `config` data key.

**Common fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `type` | yes | `psql`, `s3`, or `pvc` |
| `schedule` | yes | Cron expression (e.g., `"0 2 * * *"` for daily at 2 AM) |
| `repositoryRef` | yes | Repository reference (`namespace/name`) |
| `suspend` | no | Set to `true` to pause scheduling |
| `retention` | no | Retention policy (see below) |

**Retention policy:**

```yaml
retention:
  keepLast: 7       # Keep the last N snapshots
  keepDaily: 30     # Keep one snapshot per day for N days
  keepWeekly: 12    # Keep one snapshot per week for N weeks
  keepMonthly: 6    # Keep one snapshot per month for N months
```

## Backup Types

### PostgreSQL (`psql`)

Backs up a PostgreSQL database using `pg_dump` and stores the dump in a restic repository.

**Config fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `psql.hostname` | yes | PostgreSQL host |
| `psql.port` | no | Port (default: 5432) |
| `psql.username` | yes | Database user |
| `psql.database` | yes | Database name |

**Credentials:** Create a Secret named `<job-name>-psql-credentials` with a `password` key containing the database password.

**Apply:**

```bash
kubectl apply -f examples/hosts/s3-host.yaml
kubectl apply -f examples/repositories/repository.yaml
kubectl apply -f examples/backupjobs/psql-credentials.yaml
kubectl apply -f examples/backupjobs/psql-backup.yaml
```

### S3 (`s3`)

Mirrors an S3-compatible bucket into a restic repository.

**Config fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `s3.endpoint` | yes | S3 endpoint |
| `s3.bucket` | yes | Source bucket name |

**Credentials:** Create a Secret named `<job-name>-s3-credentials` with `accessKey` and `secretKey` keys for the **source** bucket credentials.

**Apply:**

```bash
kubectl apply -f examples/hosts/s3-host.yaml
kubectl apply -f examples/repositories/repository.yaml
kubectl apply -f examples/backupjobs/s3-credentials.yaml
kubectl apply -f examples/backupjobs/s3-backup.yaml
```

### PVC (`pvc`)

Creates a VolumeSnapshot of a PersistentVolumeClaim, mounts the snapshot as a temporary volume, and backs up the files into a restic repository.

**Config fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `pvc.claimName` | yes | Name of the PVC to back up |
| `pvc.volumeSnapshotClassName` | no | VolumeSnapshotClass to use |
| `pvc.snapshotRetention` | no | Number of VolumeSnapshots to retain |

**Prerequisites:**
- A CSI driver with snapshot support installed in the cluster
- A `VolumeSnapshotClass` resource

**Apply:**

```bash
kubectl apply -f examples/hosts/s3-host.yaml
kubectl apply -f examples/repositories/repository.yaml
kubectl apply -f examples/backupjobs/pvc-backup.yaml
```

## Full Example: End-to-End Setup

Set up a complete backup pipeline backing up a PostgreSQL database to S3:

```bash
# 1. Create the namespace
kubectl create namespace cryo

# 2. Create a repository host (where backups are stored)
kubectl apply -f examples/hosts/s3-host.yaml

# 3. Create a repository (restic repo within the host)
kubectl apply -f examples/repositories/repository.yaml

# 4. Create the database credentials
kubectl apply -f examples/backupjobs/psql-credentials.yaml

# 5. Create the backup job
kubectl apply -f examples/backupjobs/psql-backup.yaml
```

The controller will pick up the ConfigMap, schedule the backup according to the cron expression, and create a Kubernetes Job for each execution.

You can trigger a backup immediately via the API:

```bash
curl -X POST http://localhost:8080/api/v1/backupjobs/cryo/psql-backup/trigger
```

Or use the web UI to monitor and manage backup jobs.

## Notes

- All examples use the `cryo` namespace. Adjust to match your deployment.
- Use `stringData` in Secrets for readability. Kubernetes will base64-encode the values automatically.
- Repository host credentials are inherited by all repositories referencing that host.
- The controller automatically manages credential secret references in backup job ConfigMaps.
