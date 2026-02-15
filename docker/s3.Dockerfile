FROM ubuntu:24.04

ARG TARGETARCH

# Install dependencies
RUN apt update && \
    apt install -y ca-certificates curl bzip2 && \
    # Install MinIO client (mc)
    curl -L "https://dl.min.io/client/mc/release/linux-${TARGETARCH}/mc" -o /usr/local/bin/mc && \
    chmod +x /usr/local/bin/mc && \
    # Install latest restic from GitHub releases
    RESTIC_VERSION=$(curl -s https://api.github.com/repos/restic/restic/releases/latest | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/') && \
    curl -L "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH}.bz2" -o restic.bz2 && \
    bunzip2 restic.bz2 && \
    chmod +x restic && \
    mv restic /usr/local/bin/ && \
    # Clean up
    rm -rf /var/lib/apt/lists/*

COPY restic-s3.sh /scripts/restic-s3.sh
RUN chmod +x /scripts/restic-s3.sh

ENTRYPOINT ["/scripts/restic-s3.sh"]
