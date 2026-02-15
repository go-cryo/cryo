
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

start:
  ./hack/start.sh

stop:
  ./hack/stop.sh

test:
  go test -v -race -count=1 ./...

test-e2e:
  go test -v -tags=e2e -count=1 -timeout=15m ./test/e2e/...

test-all:
  go test -v -race -count=1 ./...
  go test -v -tags=e2e -count=1 -timeout=15m ./test/e2e/...