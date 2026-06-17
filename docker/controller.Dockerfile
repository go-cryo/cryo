FROM oven/bun:1.3.9 AS ui

WORKDIR /usr/src

COPY ui/package.json /usr/src/package.json
COPY ui/bun.lock /usr/src/bun.lock
COPY ui/.npmrc /usr/src/.npmrc
COPY ui/public /usr/src/public
COPY ui/src /usr/src/src
COPY ui/index.html /usr/src/index.html
COPY ui/postcss.config.js /usr/src/postcss.config.js
COPY ui/quasar.config.ts /usr/src/quasar.config.ts
COPY ui/tsconfig.json /usr/src/tsconfig.json

RUN bun install --frozen-lockfile
RUN bun run build


FROM golang:1.25.7 AS build

ARG VERSION=development

WORKDIR /usr/src
COPY go.mod /usr/src/go.mod
COPY go.sum /usr/src/go.sum

RUN go mod download

COPY cmd /usr/src/cmd
COPY internal /usr/src/internal

COPY --from=ui /usr/src/dist/spa /usr/src/internal/web/html

RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /usr/app/controller ./cmd/controller


FROM alpine:3.23.3

ARG TARGETARCH
ARG RESTIC_VERSION=0.18.1

# The controller shells out to restic for repository Check/Init and snapshot
# listing, so the binary must be present on the image.
RUN apk add --no-cache ca-certificates curl bzip2 && \
    curl -fL "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH}.bz2" -o /tmp/restic.bz2 && \
    bunzip2 /tmp/restic.bz2 && \
    install -m 0755 /tmp/restic /usr/local/bin/restic && \
    rm -f /tmp/restic && \
    apk del curl bzip2

WORKDIR /usr/app
RUN chown -R 1000:1000 /usr/app

COPY --from=build --chown=1000:1000 /usr/app/controller /usr/app/controller

USER 1000:1000
ENTRYPOINT ["/usr/app/controller"]
