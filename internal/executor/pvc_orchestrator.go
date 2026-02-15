package executor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/go-cryo/cryo/internal/settings"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var volumeSnapshotGVR = schema.GroupVersionResource{
	Group:    "snapshot.storage.k8s.io",
	Version:  "v1",
	Resource: "volumesnapshots",
}

type PVCOrchestrator struct {
	clientSet        kubernetes.Interface
	dynamicClient    dynamic.Interface
	repoProvider     repository.Provider
	pvcBackupImage   string
	settingsProvider settings.Provider
}

func NewPVCOrchestrator(clientSet kubernetes.Interface, dynamicClient dynamic.Interface, repoProvider repository.Provider, pvcBackupImage string, settingsProvider settings.Provider) *PVCOrchestrator {
	return &PVCOrchestrator{
		clientSet:        clientSet,
		dynamicClient:    dynamicClient,
		repoProvider:     repoProvider,
		pvcBackupImage:   pvcBackupImage,
		settingsProvider: settingsProvider,
	}
}

func (o *PVCOrchestrator) Run(ctx context.Context, job *backupjob.BackupJob, jobName string) error {
	pvcCfg := job.PVC
	namespace := job.Namespace

	repoParts := strings.SplitN(job.RepositoryRef, "/", 2)
	if len(repoParts) != 2 {
		return fmt.Errorf("invalid repositoryRef format: %s", job.RepositoryRef)
	}

	repo, err := o.repoProvider.Get(ctx, repoParts[0], repoParts[1])
	if err != nil {
		return fmt.Errorf("resolving repository %s: %w", job.RepositoryRef, err)
	}

	// Step 1: Create VolumeSnapshot
	snapshotName := jobName + "-snap"
	log.Info().Str("snapshot", snapshotName).Str("pvc", pvcCfg.ClaimName).Msg("creating volume snapshot")

	snapshot := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1",
			"kind":       "VolumeSnapshot",
			"metadata": map[string]interface{}{
				"name":      snapshotName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"go-cryo.github.com/backup-job":           job.Name,
					"go-cryo.github.com/backup-job-namespace": namespace,
				},
			},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"persistentVolumeClaimName": pvcCfg.ClaimName,
				},
			},
		},
	}

	if pvcCfg.VolumeSnapshotClassName != "" {
		spec := snapshot.Object["spec"].(map[string]interface{})
		spec["volumeSnapshotClassName"] = pvcCfg.VolumeSnapshotClassName
	}

	_, err = o.dynamicClient.Resource(volumeSnapshotGVR).Namespace(namespace).Create(ctx, snapshot, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating volume snapshot %s: %w", snapshotName, err)
	}

	// Step 2: Wait for snapshot ready
	if err := o.waitForSnapshotReady(ctx, namespace, snapshotName); err != nil {
		return fmt.Errorf("waiting for volume snapshot ready: %w", err)
	}
	log.Info().Str("snapshot", snapshotName).Msg("volume snapshot is ready")

	// Step 3: Get source PVC size and storage class
	sourcePVC, err := o.clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcCfg.ClaimName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting source PVC %s: %w", pvcCfg.ClaimName, err)
	}
	storageSize := sourcePVC.Spec.Resources.Requests[corev1.ResourceStorage]

	// Step 4: Create temp PVC from snapshot
	tempPVCName := jobName + "-tmp"
	log.Info().Str("pvc", tempPVCName).Msg("creating temporary PVC from snapshot")

	apiGroup := "snapshot.storage.k8s.io"
	tempPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tempPVCName,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			// Use ReadWriteOnce since many provisioners don't support ReadOnlyMany.
			// The pod spec mounts the volume read-only regardless.
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
			DataSource: &corev1.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     "VolumeSnapshot",
				Name:     snapshotName,
			},
			// Use the same storage class as the source PVC so the correct
			// CSI driver provisions the volume from the snapshot.
			StorageClassName: sourcePVC.Spec.StorageClassName,
		},
	}

	_, err = o.clientSet.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, tempPVC, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating temp PVC %s: %w", tempPVCName, err)
	}
	log.Info().Str("pvc", tempPVCName).Msg("temporary PVC created, binding deferred to pod scheduling")

	// Step 6: Create backup job
	image := job.Image
	if image == "" {
		image = o.pvcBackupImage
	}

	envVars := repoSecretEnvVars(repo)

	envVars = append(envVars, retentionEnvVars(job.Retention)...)
	envVars = append(envVars, corev1.EnvVar{Name: "BACKUP_HOST", Value: pvcCfg.ClaimName + "-backup"})

	var backoffLimit int32 = 0
	ttl := int32(604800) // 7 days default
	if o.settingsProvider != nil {
		if s, err := o.settingsProvider.Get(ctx); err == nil && s.JobTTLSeconds > 0 {
			ttl = s.JobTTLSeconds
		}
	}
	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"go-cryo.github.com/backup-job":           job.Name,
				"go-cryo.github.com/backup-job-namespace": namespace,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "backup",
							Image:           image,
							ImagePullPolicy: corev1.PullAlways,
							Env:             envVars,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: tempPVCName,
									ReadOnly:  true,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = o.clientSet.BatchV1().Jobs(namespace).Create(ctx, k8sJob, metav1.CreateOptions{})
	if err != nil {
		o.cleanupTempPVC(ctx, namespace, tempPVCName)
		return fmt.Errorf("creating backup job %s: %w", jobName, err)
	}

	// Step 7: Wait for job completion
	err = o.waitForJobCompletion(ctx, namespace, jobName)

	// Step 8: Cleanup job pods and temp PVC
	o.cleanupJobPods(ctx, namespace, jobName)
	o.cleanupTempPVC(ctx, namespace, tempPVCName)

	if err != nil {
		return err
	}

	// Step 9: Prune old snapshots
	if pvcCfg.SnapshotRetention > 0 {
		o.pruneSnapshots(ctx, namespace, job.Name, pvcCfg.SnapshotRetention)
	}

	return nil
}

func (o *PVCOrchestrator) waitForSnapshotReady(ctx context.Context, namespace, name string) error {
	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for volume snapshot %s to be ready", name)
		case <-ticker.C:
			snap, err := o.dynamicClient.Resource(volumeSnapshotGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue
			}

			status, found, err := unstructured.NestedMap(snap.Object, "status")
			if err != nil || !found {
				continue
			}

			readyToUse, found, err := unstructured.NestedBool(status, "readyToUse")
			if err != nil || !found {
				continue
			}

			if readyToUse {
				return nil
			}
		}
	}
}

func (o *PVCOrchestrator) waitForJobCompletion(ctx context.Context, namespace, jobName string) error {
	watcher, err := o.clientSet.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + jobName,
	})
	if err != nil {
		return fmt.Errorf("watching job %s/%s: %w", namespace, jobName, err)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		job, ok := event.Object.(*batchv1.Job)
		if !ok {
			continue
		}

		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				return nil
			}
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				return fmt.Errorf("job %s/%s failed: %s", namespace, jobName, condition.Message)
			}
		}
	}

	return fmt.Errorf("job %s/%s watch ended without completion", namespace, jobName)
}

func (o *PVCOrchestrator) cleanupJobPods(ctx context.Context, namespace, jobName string) {
	labelSelector := fmt.Sprintf("job-name=%s", jobName)
	err := o.clientSet.CoreV1().Pods(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Warn().Err(err).Str("job", jobName).Msg("failed to cleanup job pods")
	} else {
		log.Debug().Str("job", jobName).Msg("cleaned up job pods")
	}
}

func (o *PVCOrchestrator) cleanupTempPVC(ctx context.Context, namespace, name string) {
	err := o.clientSet.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Warn().Err(err).Str("pvc", name).Msg("failed to cleanup temporary PVC")
	} else {
		log.Info().Str("pvc", name).Msg("cleaned up temporary PVC")
	}
}

func (o *PVCOrchestrator) pruneSnapshots(ctx context.Context, namespace, jobName string, retention int) {
	labelSelector := fmt.Sprintf("go-cryo.github.com/backup-job=%s", jobName)

	snapshots, err := o.dynamicClient.Resource(volumeSnapshotGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Warn().Err(err).Str("job", jobName).Msg("failed to list snapshots for pruning")
		return
	}

	if len(snapshots.Items) <= retention {
		return
	}

	// Sort by creation timestamp, newest first
	sort.Slice(snapshots.Items, func(i, j int) bool {
		return snapshots.Items[i].GetCreationTimestamp().After(snapshots.Items[j].GetCreationTimestamp().Time)
	})

	for _, snap := range snapshots.Items[retention:] {
		snapName := snap.GetName()
		err := o.dynamicClient.Resource(volumeSnapshotGVR).Namespace(namespace).Delete(ctx, snapName, metav1.DeleteOptions{})
		if err != nil {
			log.Warn().Err(err).Str("snapshot", snapName).Msg("failed to prune old volume snapshot")
		} else {
			log.Info().Str("snapshot", snapName).Msg("pruned old volume snapshot")
		}
	}
}
