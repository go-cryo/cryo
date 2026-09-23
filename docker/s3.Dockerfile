FROM alpine:3.23.3

ARG TARGETARCH
ARG RESTIC_VERSION=0.18.1

# mc (MinIO client) and restic are statically linked, so they run on musl/alpine.
# dl.min.io stopped serving mc binaries (HTTP 410), so copy it from MinIO's last
# published multi-arch image instead.
COPY --from=quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z /usr/bin/mc /usr/local/bin/mc
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
