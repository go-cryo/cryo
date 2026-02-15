#!/bin/bash

# Script to backup PVC data using restic
# The PVC should be mounted at /data by the controller
# Required environment variables:
# - RESTIC_REPOSITORY
# - RESTIC_PASSWORD
# - BACKUP_HOST (hostname for restic snapshots grouping)
#
# Optional environment variables:
# - KEEP_LAST (default: 1)
# - KEEP_DAILY (default: 7)
# - KEEP_WEEKLY (default: 4)
# - KEEP_MONTHLY (default: 12)
# - AWS_ACCESS_KEY_ID (for S3 repos)
# - AWS_SECRET_ACCESS_KEY (for S3 repos)
# - AWS_DEFAULT_REGION (for S3 repos)

# Validate required environment variables
REQUIRED_VARS=(RESTIC_REPOSITORY RESTIC_PASSWORD BACKUP_HOST)

for var in "${REQUIRED_VARS[@]}"; do
  if [ -z "${!var}" ]; then
    echo "Error: Required environment variable $var is not set"
    exit 1
  fi
done

# Cleanup function
cleanup() {
  echo "Cleaning up restic cache"
  restic cache --cleanup
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
    exit 1
  fi
else
  echo "Repository exists and is accessible"
fi

echo "Running backup to restic repository"
echo "Backing up /data with host: $BACKUP_HOST"
# Use --host to group snapshots by a consistent identifier instead of the pod hostname
# This ensures snapshots from different pod instances are treated as one logical backup set
restic backup --host "$BACKUP_HOST" /data

if [ $? -eq 0 ]; then
  echo "Backup successful"
else
  echo "Backup failed"
  exit 1
fi

echo "Forgetting old snapshots"
# Use --host and --group-by host to ensure forget policy applies across all pod instances
restic forget --host "$BACKUP_HOST" --group-by host \
  --keep-last ${KEEP_LAST:-1} \
  --keep-daily ${KEEP_DAILY:-7} \
  --keep-weekly ${KEEP_WEEKLY:-4} \
  --keep-monthly ${KEEP_MONTHLY:-12} \
  --prune

echo "Snapshots after cleanup"
restic snapshots --host "$BACKUP_HOST"

exit 0
