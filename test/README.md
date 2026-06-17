# Testing

Cryo has three test layers, all runnable locally against a KIND cluster.

| Layer | Command | Cluster | What it proves |
|-------|---------|---------|----------------|
| Unit | `just test` | none | Provider/config/scheduler/auth logic against fake clients |
| Go E2E | `just test-e2e` | KIND + RustFS | Real psql/s3/pvc backups via restic + the auth-protected API |
| Playwright UI | `just test-ui` | KIND + RustFS + in-cluster controller | The full backup lifecycle driven through the browser |

## Prerequisites

`docker`, `kind`, `kubectl`, `helm`, `go`, and `bun`. Docker needs a few GB of
free disk for the cluster, RustFS, and backup-job images.

## The local loop

```bash
just cluster-up      # KIND cluster + local registry + CSI + backup images
just test            # unit tests (no cluster needed)
just test-e2e        # Go end-to-end tests
just test-ui         # Playwright UI tests
just cluster-down    # tear everything down
```

`just test-all` runs all three layers in sequence.

## How it fits together

- **RustFS** is the S3 backup target. It runs in-cluster (`test/e2e/manifests/rustfs.yaml`)
  because backup jobs execute as Kubernetes Jobs and must reach it over the cluster
  network. It serves both the restic repository backend (`cryo-repo`) and the S3
  backup source bucket (`test-source`). restic talks to it unchanged — it is a
  drop-in MinIO replacement.
- **BasicAuth** is enabled for every test run. The Go E2E suite bootstraps an admin
  user, logs in, and routes all API calls through an authenticated cookie jar. The
  Playwright suite exercises the real login form.
- **Backup-job images** (`cryo-{psql,s3,pvc}`) run with `imagePullPolicy: Always`,
  so they are served from a local registry (`localhost:5001`, mirrored into KIND via
  `test/e2e/kind-config.yaml`) rather than `kind load`.

### Go E2E (`test/e2e/`, build tag `e2e`)

`TestMain` (`setup_test.go`) creates a throwaway namespace, deploys RustFS +
PostgreSQL + a test PVC, wires the providers/scheduler/executor and an
`httptest` server with BasicAuth, logs in, and runs the suite. It needs the
`cryo-test` cluster from `just cluster-up` to be reachable via the current
kubecontext.

### Playwright UI (`ui/tests/e2e/`)

`hack/playwright-up.sh` builds the controller image (UI embedded + restic),
deploys it **in-cluster** in the `cryo-ui` namespace with BasicAuth enabled,
deploys + seeds RustFS, port-forwards the controller to `localhost:8080`, and
writes the admin credentials to `ui/.e2e-env`. Running the controller in-cluster
lets snapshot browsing (`restic snapshots`, executed by the controller process)
reach RustFS natively.

`backup-flow.spec.ts` then drives the browser: log in → create a RustFS host →
repository → S3 backup job → trigger it → wait for the run to succeed → browse
the resulting snapshot → log out. `hack/playwright-down.sh` removes the
port-forward and namespace.
