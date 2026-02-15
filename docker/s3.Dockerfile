FROM ubuntu:noble-20260113

ARG TARGETARCH
ARG RESTIC_VERSION=0.18.1

# Install dependencies
RUN apt update && \
    apt install -y ca-certificates curl bzip2 && \
    # Install MinIO client (mc)
    curl -fL "https://dl.min.io/client/mc/release/linux-${TARGETARCH}/mc" -o /usr/local/bin/mc && \
    chmod +x /usr/local/bin/mc && \
    # Install restic
    curl -fL "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH}.bz2" -o restic.bz2 && \
    bunzip2 restic.bz2 && \
    chmod +x restic && \
    mv restic /usr/local/bin/ && \
    # Clean up
    rm -rf /var/lib/apt/lists/*

COPY restic-s3.sh /scripts/restic-s3.sh
RUN chmod +x /scripts/restic-s3.sh

ENTRYPOINT ["/scripts/restic-s3.sh"]
