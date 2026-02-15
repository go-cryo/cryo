#!/bin/bash

# Script to backup PostgreSQL database using pg_dump and restic
# Required environment variables:
# - RESTIC_REPOSITORY
# - RESTIC_PASSWORD
# - POSTGRES_HOSTNAME
# - POSTGRES_USERNAME
# - POSTGRES_PASSWORD
# - POSTGRES_DATABASE
#
# Optional environment variables:
# - POSTGRES_PORT (defaults to 5432)
# - TEMP_DIR (defaults to /tmp/psql-backup)

# Temporary directory for database dumps
TEMP_DIR="${TEMP_DIR:-/tmp/psql-backup}"

# Validate required environment variables
REQUIRED_VARS=(RESTIC_REPOSITORY RESTIC_PASSWORD POSTGRES_HOSTNAME POSTGRES_USERNAME POSTGRES_PASSWORD POSTGRES_DATABASE)

for var in "${REQUIRED_VARS[@]}"; do
  if [ -z "${!var}" ]; then
    echo "Error: Required environment variable $var is not set"
    exit 1
  fi
done

# Set default port if not specified
POSTGRES_PORT="${POSTGRES_PORT:-5432}"

# Create temporary directory
mkdir -p $TEMP_DIR

# Cleanup function
cleanup() {
  echo "Cleaning up temporary files"
  rm -rf $TEMP_DIR/*
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Check if restic repository exists, initialize if not
echo "Checking restic repository: $RESTIC_REPOSITORY"
restic snapshots &>/dev/null
if [ $? -ne 0 ]; then
  echo "Repository does not exist or is not accessible. Initializing..."
  restic init
  if [ $? -eq 0 ]; then
    echo "Repository initialized successfully"
  else
    echo "Failed to initialize repository"
    cleanup
    exit 1
  fi
else
  echo "Repository exists and is accessible"
fi

# Use a consistent filename to leverage restic's incremental backup and deduplication
# Restic will efficiently handle deltas between backups
DUMP_FILE="$TEMP_DIR/${POSTGRES_DATABASE}.sql"

echo "Creating PostgreSQL dump: $DUMP_FILE"
echo "Database: $POSTGRES_DATABASE on $POSTGRES_HOSTNAME:$POSTGRES_PORT"
echo "Using plain SQL format to leverage restic's incremental backup capabilities"

# Export password for pg_dump
export PGPASSWORD="$POSTGRES_PASSWORD"

# Create database dump in plain SQL format for better deduplication
pg_dump -h "$POSTGRES_HOSTNAME" -p "$POSTGRES_PORT" -U "$POSTGRES_USERNAME" -d "$POSTGRES_DATABASE" -f "$DUMP_FILE"

if [ $? -eq 0 ]; then
  echo "Database dump created successfully"
  echo "Dump file size: $(du -h $DUMP_FILE | cut -f1)"
else
  echo "Database dump failed"
  cleanup
  exit 1
fi

# Unset password
unset PGPASSWORD

echo "Running backup to restic repository"
# Use --host to group snapshots by a consistent identifier instead of the pod hostname
# This ensures snapshots from different pod instances are treated as one logical backup set
restic backup --host "${POSTGRES_DATABASE}-backup" $TEMP_DIR

if [ $? -eq 0 ]; then
  echo "Backup successful"
else
  echo "Backup failed"
  cleanup
  exit 1
fi

echo "Forgetting old snapshots"
# Use --host and --group-by host to ensure forget policy applies across all pod instances
restic forget --host "${POSTGRES_DATABASE}-backup" --group-by host \
  --keep-last ${KEEP_LAST:-1} \
  --keep-daily ${KEEP_DAILY:-7} \
  --keep-weekly ${KEEP_WEEKLY:-4} \
  --keep-monthly ${KEEP_MONTHLY:-12} \
  --prune

echo "Snapshots after cleanup"
restic snapshots --host "${POSTGRES_DATABASE}-backup"

echo "Cleaning up restic cache"
restic cache --cleanup

exit 0