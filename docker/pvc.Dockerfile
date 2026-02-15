FROM alpine:3.23

ARG TARGETARCH

RUN apk add --no-cache ca-certificates curl bash bzip2 && \
    RESTIC_VERSION=$(curl -s https://api.github.com/repos/restic/restic/releases/latest | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/') && \
    curl -L "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH}.bz2" -o restic.bz2 && \
    bunzip2 restic.bz2 && \
    chmod +x restic && \
    mv restic /usr/local/bin/ && \
    apk del curl bzip2

COPY restic-pvc.sh /scripts/restic-pvc.sh
RUN chmod +x /scripts/restic-pvc.sh

ENTRYPOINT ["/scripts/restic-pvc.sh"]
