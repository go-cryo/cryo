FROM ubuntu:noble-20260113

ARG TARGETARCH
ARG RESTIC_VERSION=0.18.1

# Install dependencies and add PostgreSQL official repository
RUN apt update && \
    apt install -y ca-certificates curl gnupg lsb-release bzip2 && \
    # Add PostgreSQL official APT repository
    curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/keyrings/postgresql-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/postgresql-keyring.gpg] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list && \
    # Update and install latest PostgreSQL client
    apt update && \
    apt install -y postgresql-client && \
    # Install restic
    curl -fL "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH}.bz2" -o restic.bz2 && \
    bunzip2 restic.bz2 && \
    chmod +x restic && \
    mv restic /usr/local/bin/ && \
    # Clean up
    rm -rf /var/lib/apt/lists/*

COPY restic-psql.sh /scripts/restic-psql.sh
RUN chmod +x /scripts/restic-psql.sh

ENTRYPOINT ["/scripts/restic-psql.sh"]
