# MinIO stopped publishing mc: dl.min.io answers 410, docker.io/minio/mc and
# quay.io/minio/mc are gone (401). Build the pinned release from source instead.
FROM golang:1.25-alpine AS mc
ARG MC_VERSION=RELEASE.2025-08-13T08-35-41Z
RUN CGO_ENABLED=0 go install -trimpath -ldflags "-s -w" github.com/minio/mc@${MC_VERSION}

FROM alpine:3.23.3

ARG TARGETARCH
ARG RESTIC_VERSION=0.18.1

# mc and restic are statically linked, so they run on musl/alpine.
COPY --from=mc /go/bin/mc /usr/local/bin/mc
RUN mc --version

RUN apk add --no-cache ca-certificates bash curl bzip2 && \
    curl -fL "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH}.bz2" -o restic.bz2 && \
    bunzip2 restic.bz2 && \
    chmod +x restic && \
    mv restic /usr/local/bin/ && \
    apk del curl bzip2

COPY restic-s3.sh /scripts/restic-s3.sh
RUN chmod +x /scripts/restic-s3.sh

ENTRYPOINT ["/scripts/restic-s3.sh"]
