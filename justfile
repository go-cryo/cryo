
# pushes all changes to the main branch
push +COMMIT_MESSAGE:
  go mod tidy
  git add .
  git commit -m "{{COMMIT_MESSAGE}}"
  git pull origin main
  git push origin main

tag +TAG_NAME:
  git tag {{TAG_NAME}}
  git push origin {{TAG_NAME}}

# start the Go API with live reload
air:
  air -c .air.toml

generate:
  go generate ./...

# create the local KIND test cluster (registry + CSI + backup images)
cluster-up:
  ./hack/cluster-up.sh

# delete the local KIND test cluster and registry
cluster-down:
  ./hack/cluster-down.sh

# aliases kept for backwards compatibility
start: cluster-up
stop: cluster-down

# unit tests (no cluster required)
test:
  go test -v -race -count=1 ./...

# Go end-to-end tests against the local cluster + RustFS with auth (needs `just cluster-up`)
test-e2e:
  go test -v -tags=e2e -count=1 -timeout=15m ./test/e2e/...

# Playwright UI end-to-end tests (deploys the controller in-cluster, needs `just cluster-up`)
test-ui:
  #!/usr/bin/env bash
  set -euo pipefail
  ./hack/playwright-up.sh
  cd ui
  set -a; source .e2e-env; set +a
  bun run test:e2e

# tear down the Playwright stack (port-forward + cryo-ui namespace)
test-ui-down:
  ./hack/playwright-down.sh

# the full local loop: unit + Go e2e + Playwright UI
test-all:
  go test -v -race -count=1 ./...
  go test -v -tags=e2e -count=1 -timeout=15m ./test/e2e/...
  just test-ui
