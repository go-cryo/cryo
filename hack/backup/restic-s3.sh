#!/bin/bash

# Script to backup S3 bucket using MinIO client (mc) and restic
# Required environment variables:
# - RESTIC_REPOSITORY
# - RESTIC_PASSWORD
# - S3_ENDPOINT
# - S3_ACCESS_KEY
# - S3_SECRET_KEY
# - S3_BUCKET
#
# Optional environment variables:
# - TEMP_DIR (defaults to /tmp/s3-backup)

# Temporary directory for S3 mirror
TEMP_DIR="${TEMP_DIR:-/tmp/s3-backup}"

# Validate required environment variables
REQUIRED_VARS=(RESTIC_REPOSITORY RESTIC_PASSWORD S3_ENDPOINT S3_ACCESS_KEY S3_SECRET_KEY S3_BUCKET)

for var in "${REQUIRED_VARS[@]}"; do
  if [ -z "${!var}" ]; then
    echo "Error: Required environment variable $var is not set"
    exit 1
  fi
done

# Create temporary directory
mkdir -p $TEMP_DIR

# Cleanup function
cleanup() {
  echo "Cleaning up temporary files"
  rm -rf $TEMP_DIR/*
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Configure MinIO client alias
echo "Configuring MinIO client for S3 endpoint: $S3_ENDPOINT"
mc alias set s3backup "$S3_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY"

if [ $? -eq 0 ]; then
  echo "MinIO client configured successfully"
else
  echo "Failed to configure MinIO client"
  cleanup
  exit 1
fi

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

# Mirror S3 bucket to local temporary directory
echo "Mirroring S3 bucket to temporary directory: $TEMP_DIR"
echo "Bucket: s3backup/$S3_BUCKET"

mc mirror --quiet "s3backup/$S3_BUCKET" "$TEMP_DIR"

if [ $? -eq 0 ]; then
  echo "S3 bucket mirrored successfully"
  echo "Mirror directory size: $(du -sh $TEMP_DIR | cut -f1)"
else
  echo "S3 bucket mirror failed"
  cleanup
  exit 1
fi

echo "Running backup to restic repository"
# Use --host to group snapshots by a consistent identifier instead of the pod hostname
# This ensures snapshots from different pod instances are treated as one logical backup set
restic backup --host "${S3_BUCKET}-backup" $TEMP_DIR

if [ $? -eq 0 ]; then
  echo "Backup successful"
else
  echo "Backup failed"
  cleanup
  exit 1
fi

echo "Forgetting old snapshots"
# Use --host and --group-by host to ensure forget policy applies across all pod instances
restic forget --host "${S3_BUCKET}-backup" --group-by host \
  --keep-last ${KEEP_LAST:-1} \
  --keep-daily ${KEEP_DAILY:-7} \
  --keep-weekly ${KEEP_WEEKLY:-4} \
  --keep-monthly ${KEEP_MONTHLY:-12} \
  --prune

echo "Snapshots after cleanup"
restic snapshots --host "${S3_BUCKET}-backup"

echo "Cleaning up restic cache"
restic cache --cleanup

exit 0