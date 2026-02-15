this is the cronjob we want to replace with this backup controller:
```
{{- range .Values.backupJobs }}
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: pvc-backup-{{ .name }}
  namespace: {{ .namespace }}
  labels:
    app.kubernetes.io/name: pvc-backup-{{ .name }}
    app.kubernetes.io/component: pvc-backup
spec:
  schedule: {{ .schedule | quote }}
  successfulJobsHistoryLimit: {{ $.Values.backupSettings.successfulJobsHistoryLimit }}
  failedJobsHistoryLimit: {{ $.Values.backupSettings.failedJobsHistoryLimit }}
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      backoffLimit: {{ $.Values.backupSettings.backoffLimit }}
      activeDeadlineSeconds: {{ $.Values.backupSettings.activeDeadlineSeconds }}
      ttlSecondsAfterFinished: {{ $.Values.backupSettings.ttlSecondsAfterFinished }}
      template:
        metadata:
          labels:
            app.kubernetes.io/name: pvc-backup-{{ .name }}
            app.kubernetes.io/component: pvc-backup
        spec:
          restartPolicy: {{ $.Values.backupSettings.restartPolicy }}
          serviceAccountName: pvc-backup-{{ .name }}
          containers:
            - name: orchestrator
              image: "{{ $.Values.initImage.repository }}:{{ $.Values.initImage.tag }}"
              imagePullPolicy: {{ $.Values.initImage.pullPolicy }}
              env:
                - name: SNAPSHOT_NAME
                  value: "pvc-backup-{{ .name }}-snap"
                - name: RESTORE_PVC_NAME
                  value: "pvc-backup-{{ .name }}-restore"
                - name: SOURCE_PVC_NAME
                  value: {{ .pvcName | quote }}
                - name: NAMESPACE
                  value: {{ .namespace | quote }}
                - name: SNAPSHOT_CLASS
                  value: {{ $.Values.snapshot.className | quote }}
                - name: STORAGE_SIZE
                  value: {{ .storage | quote }}
                - name: BACKUP_JOB_NAME
                  value: "pvc-backup-{{ .name }}-runner"
                - name: BACKUP_IMAGE
                  value: "{{ $.Values.image.repository }}:{{ $.Values.image.tag }}"
                - name: BACKUP_IMAGE_PULL_SECRET
                  value: {{ $.Values.image.pullSecretName | quote }}
                - name: SERVICE_ACCOUNT_NAME
                  value: "pvc-backup-{{ .name }}"
                - name: RESTIC_REPOSITORY
                  value: "{{ $.Values.restic.s3Endpoint }}{{ $.Values.restic.basePath }}/{{ .name }}"
                - name: SECRET_NAME
                  value: "pvc-backup-{{ .name }}"
                - name: S3_CREDENTIALS_SECRET
                  value: {{ $.Values.restic.s3CredentialsSecret | quote }}
                - name: BACKUP_NAME
                  value: {{ .name | quote }}
                - name: RESOURCE_REQUESTS_CPU
                  value: {{ $.Values.resources.requests.cpu | quote }}
                - name: RESOURCE_REQUESTS_MEMORY
                  value: {{ $.Values.resources.requests.memory | quote }}
                - name: RESOURCE_LIMITS_MEMORY
                  value: {{ $.Values.resources.limits.memory | quote }}
              command:
                - /bin/bash
                - -c
                - |
                  set -e
                  
                  echo "=== PVC Backup Orchestrator ==="
                  echo "Source PVC: ${SOURCE_PVC_NAME}"
                  echo "Snapshot: ${SNAPSHOT_NAME}"
                  echo "Restore PVC: ${RESTORE_PVC_NAME}"
                  
                  # Cleanup any existing temporary resources from previous runs
                  echo "Cleaning up any existing temporary resources..."
                  kubectl delete job ${BACKUP_JOB_NAME} -n ${NAMESPACE} --ignore-not-found=true || true
                  kubectl delete pvc ${RESTORE_PVC_NAME} -n ${NAMESPACE} --ignore-not-found=true --wait=true || true
                  kubectl delete volumesnapshot ${SNAPSHOT_NAME} -n ${NAMESPACE} --ignore-not-found=true --wait=true || true
                  
                  # Wait a moment for cleanup to complete
                  sleep 5
                  
                  # Step 1: Create VolumeSnapshot
                  echo "Creating VolumeSnapshot ${SNAPSHOT_NAME} from PVC ${SOURCE_PVC_NAME}..."
                  
                  cat <<EOF | kubectl apply -f -
                  apiVersion: snapshot.storage.k8s.io/v1
                  kind: VolumeSnapshot
                  metadata:
                    name: ${SNAPSHOT_NAME}
                    namespace: ${NAMESPACE}
                    labels:
                      app.kubernetes.io/name: pvc-backup-{{ .name }}
                      app.kubernetes.io/component: pvc-backup
                      pvc-backup/source-pvc: ${SOURCE_PVC_NAME}
                      pvc-backup/temporary: "true"
                  spec:
                    volumeSnapshotClassName: ${SNAPSHOT_CLASS}
                    source:
                      persistentVolumeClaimName: ${SOURCE_PVC_NAME}
                  EOF
                  
                  echo "Waiting for VolumeSnapshot to be ready..."
                  for i in {1..120}; do
                    READY=$(kubectl get volumesnapshot ${SNAPSHOT_NAME} -n ${NAMESPACE} -o jsonpath='{.status.readyToUse}' 2>/dev/null || echo "false")
                    if [ "${READY}" = "true" ]; then
                      echo "VolumeSnapshot ${SNAPSHOT_NAME} is ready!"
                      break
                    fi
                    if [ $i -eq 120 ]; then
                      echo "ERROR: VolumeSnapshot did not become ready within timeout (20 minutes)"
                      kubectl get volumesnapshot ${SNAPSHOT_NAME} -n ${NAMESPACE} -o yaml
                      exit 1
                    fi
                    echo "Waiting for snapshot... (attempt ${i}/120)"
                    sleep 10
                  done
                  
                  # Step 2: Create restore PVC from snapshot
                  echo "Creating restore PVC ${RESTORE_PVC_NAME} from snapshot ${SNAPSHOT_NAME}..."
                  
                  cat <<EOF | kubectl apply -f -
                  apiVersion: v1
                  kind: PersistentVolumeClaim
                  metadata:
                    name: ${RESTORE_PVC_NAME}
                    namespace: ${NAMESPACE}
                    labels:
                      app.kubernetes.io/name: pvc-backup-{{ .name }}
                      app.kubernetes.io/component: pvc-backup
                      pvc-backup/temporary: "true"
                  spec:
                    accessModes:
                      - ReadOnlyMany
                    storageClassName: ceph-fs
                    resources:
                      requests:
                        storage: ${STORAGE_SIZE}
                    dataSource:
                      name: ${SNAPSHOT_NAME}
                      kind: VolumeSnapshot
                      apiGroup: snapshot.storage.k8s.io
                  EOF
                  
                  echo "Waiting for restore PVC to be bound..."
                  for i in {1..120}; do
                    STATUS=$(kubectl get pvc ${RESTORE_PVC_NAME} -n ${NAMESPACE} -o jsonpath='{.status.phase}' 2>/dev/null || echo "Pending")
                    if [ "${STATUS}" = "Bound" ]; then
                      echo "Restore PVC ${RESTORE_PVC_NAME} is bound!"
                      break
                    fi
                    if [ $i -eq 120 ]; then
                      echo "ERROR: Restore PVC did not become bound within timeout (20 minutes)"
                      kubectl get pvc ${RESTORE_PVC_NAME} -n ${NAMESPACE} -o yaml
                      kubectl get events -n ${NAMESPACE} --field-selector involvedObject.name=${RESTORE_PVC_NAME}
                      exit 1
                    fi
                    echo "Waiting for PVC to be bound... (attempt ${i}/120, status: ${STATUS})"
                    sleep 10
                  done
                  
                  # Step 3: Create and run backup job
                  echo "Creating backup job ${BACKUP_JOB_NAME}..."
                  
                  cat <<EOF | kubectl apply -f -
                  apiVersion: batch/v1
                  kind: Job
                  metadata:
                    name: ${BACKUP_JOB_NAME}
                    namespace: ${NAMESPACE}
                    labels:
                      app.kubernetes.io/name: pvc-backup-{{ .name }}
                      app.kubernetes.io/component: pvc-backup-runner
                      pvc-backup/temporary: "true"
                  spec:
                    backoffLimit: 1
                    activeDeadlineSeconds: 86400
                    ttlSecondsAfterFinished: 300
                    template:
                      metadata:
                        labels:
                          app.kubernetes.io/name: pvc-backup-{{ .name }}
                          app.kubernetes.io/component: pvc-backup-runner
                      spec:
                        restartPolicy: Never
                        serviceAccountName: ${SERVICE_ACCOUNT_NAME}
                        imagePullSecrets:
                          - name: ${BACKUP_IMAGE_PULL_SECRET}
                        containers:
                          - name: backup
                            image: ${BACKUP_IMAGE}
                            imagePullPolicy: IfNotPresent
                            command: ["/scripts/restic-pvc.sh"]
                            env:
                              - name: RESTIC_REPOSITORY
                                value: "${RESTIC_REPOSITORY}"
                              - name: RESTIC_PASSWORD
                                valueFrom:
                                  secretKeyRef:
                                    name: ${SECRET_NAME}
                                    key: BACKUP_ENCRYPTION_KEY
                              - name: AWS_ACCESS_KEY_ID
                                valueFrom:
                                  secretKeyRef:
                                    name: ${S3_CREDENTIALS_SECRET}
                                    key: AWS_ACCESS_KEY_ID
                              - name: AWS_SECRET_ACCESS_KEY
                                valueFrom:
                                  secretKeyRef:
                                    name: ${S3_CREDENTIALS_SECRET}
                                    key: AWS_SECRET_ACCESS_KEY
                              - name: BACKUP_PATH
                                value: "/data"
                              - name: BACKUP_NAME
                                value: "${BACKUP_NAME}"
                            volumeMounts:
                              - name: restore-data
                                mountPath: /data
                                readOnly: true
                            resources:
                              requests:
                                cpu: "${RESOURCE_REQUESTS_CPU}"
                                memory: "${RESOURCE_REQUESTS_MEMORY}"
                              limits:
                                memory: "${RESOURCE_LIMITS_MEMORY}"
                        volumes:
                          - name: restore-data
                            persistentVolumeClaim:
                              claimName: ${RESTORE_PVC_NAME}
                  EOF
                  
                  # Step 4: Wait for backup job to complete
                  echo "Waiting for backup job to complete..."
                  kubectl wait --for=condition=complete --timeout=86400s job/${BACKUP_JOB_NAME} -n ${NAMESPACE} || {
                    echo "ERROR: Backup job failed or timed out"
                    kubectl logs job/${BACKUP_JOB_NAME} -n ${NAMESPACE} || true
                    kubectl get job ${BACKUP_JOB_NAME} -n ${NAMESPACE} -o yaml
                    exit 1
                  }
                  
                  echo "Backup job completed successfully!"
                  kubectl logs job/${BACKUP_JOB_NAME} -n ${NAMESPACE} || true
                  
                  echo "=== PVC Backup Complete ==="
              resources:
                requests:
                  cpu: "100m"
                  memory: "128Mi"
                limits:
                  memory: "256Mi"
{{- end }}
```

Here we have the manifests that work for our development cluster for creating snapshots and mounting them to a new pvc to be mounted:

```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: demo-pvc
spec:
  storageClassName: csi-hostpath
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Gi
---
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: demo-snap
spec:
  volumeSnapshotClassName: csi-hostpath-snapclass
  source:
    persistentVolumeClaimName: demo-pvc
---
apiVersion: v1
kind: Pod
metadata:
  name: pvc-mount-test-restore
spec:
  restartPolicy: Never
  containers:
    - name: app
      image: busybox:1.36
      command: ["sh", "-c", "echo hello > /data/hello.txt && sleep 3600"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: demo-pvc-restore
```