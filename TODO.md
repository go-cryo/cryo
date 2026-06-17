# TODO

Tech debt and follow-ups spotted while building the test suites. None block the
current functionality.

## UI

- **Trigger feedback is invisible.** `POST /backupjobs/:ns/:name/trigger` returns
  `202` with an empty body, so `BackupJobDetailPage.onTrigger` sets a falsy
  `triggerResult` and the "Backup run … started" banner never renders. Either
  return the created `BackupRun` from the trigger endpoint, or show the banner
  optimistically.
- **Runs table does not live-update.** `BackupJobDetailPage` fetches runs only on
  mount / manual refresh, even though the app already has a WebSocket event stream
  (`EventObjectBackupRun`). Subscribe to it so a run's status updates without a
  manual refresh.

## Backend

- **psql backup image is heavy.** `docker/psql.Dockerfile` is still `ubuntu:noble`
  (~210 MB). `s3` and `pvc` were slimmed to Alpine; psql could follow using
  `postgresql-client` from Alpine (mind the pg_dump/server version skew).

## Done in this pass (for reference)

- Controller image now ships `restic` — snapshot listing / repo check previously
  failed wherever the host lacked it.
- Auth whitelist now exempts the real health path (`<apiBase>/health`), so liveness
  probes are not rejected when BasicAuth is enabled.
- **combinedAuth no longer hard-codes `/api/`** — it uses the configured
  `ApiBaseUrl`, closing an auth bypass for a non-default `API_BASE_URL` (Codex P2).
- **OIDC sessions are honoured by the SPA guard** — added a method-agnostic
  protected `<apiBase>/auth/session` probe and pointed the router guard at it
  instead of the BasicAuth-only `/auth/me` (Codex P1).
