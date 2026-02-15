package executor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/event"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/go-cryo/cryo/internal/settings"
	"github.com/rs/zerolog/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ExecutorOptions struct {
	PSQLBackupImage string
	S3BackupImage   string
	PVCBackupImage  string
}

type Executor struct {
	clientSet        kubernetes.Interface
	repoProvider     repository.Provider
	runStore         *RunStore
	orchestrator     *PVCOrchestrator
	options          *ExecutorOptions
	settingsProvider settings.Provider
}

func NewExecutor(clientSet kubernetes.Interface, repoProvider repository.Provider, runStore *RunStore, orchestrator *PVCOrchestrator, options *ExecutorOptions, settingsProvider settings.Provider) *Executor {
	return &Executor{
		clientSet:        clientSet,
		repoProvider:     repoProvider,
		runStore:         runStore,
		orchestrator:     orchestrator,
		options:          options,
		settingsProvider: settingsProvider,
	}
}

func (e *Executor) Execute(ctx context.Context, job *backupjob.BackupJob) (*backupjob.BackupRun, error) {
	key := job.Namespace + "/" + job.Name
	log.Info().Str("job", key).Str("type", string(job.Type)).Msg("executing backup job")

	now := time.Now()
	run := &backupjob.BackupRun{
		JobName:   job.Name,
		Namespace: job.Namespace,
		Status:    backupjob.BackupRunStatusRunning,
		StartTime: &now,
	}
	e.runStore.SetActive(run)

	event.BroadcastEvent(&event.Event{
		Object: event.EventObjectBackupRun,
		Action: event.EventActionCreate,
	})

	var err error
	switch job.Type {
	case backupjob.BackupJobTypePSQL:
		err = e.executePSQL(ctx, job, run)
	case backupjob.BackupJobTypeS3:
		err = e.executeS3(ctx, job, run)
	case backupjob.BackupJobTypePVC:
		err = e.executePVC(ctx, job, run)
	default:
		err = fmt.Errorf("unknown backup job type: %s", job.Type)
	}

	endTime := time.Now()
	run.EndTime = &endTime
	if err != nil {
		run.Status = backupjob.BackupRunStatusFailed
		run.Message = err.Error()
		log.Error().Err(err).Str("job", key).Msg("backup job execution failed")
	} else {
		run.Status = backupjob.BackupRunStatusSucceeded
		log.Info().Str("job", key).Msg("backup job execution completed")
	}

	e.runStore.ClearActive(job.Namespace, job.Name)

	event.BroadcastEvent(&event.Event{
		Object: event.EventObjectBackupRun,
		Action: event.EventActionUpdate,
	})

	return run, err
}

func (e *Executor) executePSQL(ctx context.Context, job *backupjob.BackupJob, run *backupjob.BackupRun) error {
	if job.PSQL == nil {
		return fmt.Errorf("psql config is required for psql backup job")
	}

	repoParts := strings.SplitN(job.RepositoryRef, "/", 2)
	if len(repoParts) != 2 {
		return fmt.Errorf("invalid repositoryRef format: %s", job.RepositoryRef)
	}

	repo, err := e.repoProvider.Get(ctx, repoParts[0], repoParts[1])
	if err != nil {
		return fmt.Errorf("resolving repository %s: %w", job.RepositoryRef, err)
	}

	image := job.Image
	if image == "" {
		image = e.options.PSQLBackupImage
	}

	timestamp := time.Now().Format("20060102-150405")
	jobName := fmt.Sprintf("cryo-psql-%s-%s", job.Name, timestamp)
	run.Name = jobName

	envVars := repoSecretEnvVars(repo)

	if job.PSQL.CredentialSecretRef == "" {
		return fmt.Errorf("psql credentialSecretRef is required")
	}

	psqlSecretName := job.PSQL.CredentialSecretRef
	optional := true
	envVars = append(envVars,
		corev1.EnvVar{
			Name: "POSTGRES_HOSTNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: psqlSecretName},
					Key:                  "hostname",
				},
			},
		},
		corev1.EnvVar{
			Name: "POSTGRES_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: psqlSecretName},
					Key:                  "username",
				},
			},
		},
		corev1.EnvVar{
			Name: "POSTGRES_DATABASE",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: psqlSecretName},
					Key:                  "database",
				},
			},
		},
		corev1.EnvVar{
			Name: "POSTGRES_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: psqlSecretName},
					Key:                  "password",
				},
			},
		},
		corev1.EnvVar{
			Name: "POSTGRES_PORT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: psqlSecretName},
					Key:                  "port",
					Optional:             &optional,
				},
			},
		},
	)

	envVars = append(envVars, retentionEnvVars(job.Retention)...)

	// Staging PVC
	stagingSize := "1Gi"
	stagingStorageClass := ""
	if job.PSQL.StagingSize != "" {
		stagingSize = job.PSQL.StagingSize
	}
	if job.PSQL.StagingStorageClassName != "" {
		stagingStorageClass = job.PSQL.StagingStorageClassName
	} else if e.settingsProvider != nil {
		s, err := e.settingsProvider.Get(ctx)
		if err == nil && s.DefaultStorageClassName != "" {
			stagingStorageClass = s.DefaultStorageClassName
		}
	}

	stagingPVCName := jobName + "-staging"
	if err := e.createStagingPVC(ctx, job.Namespace, stagingPVCName, stagingSize, stagingStorageClass); err != nil {
		return fmt.Errorf("creating staging PVC: %w", err)
	}
	defer e.cleanupStagingPVC(ctx, job.Namespace, stagingPVCName)

	envVars = append(envVars, corev1.EnvVar{Name: "TEMP_DIR", Value: "/staging/psql-backup"})

	volumes := []corev1.Volume{
		{
			Name: "staging",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: stagingPVCName,
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "staging",
			MountPath: "/staging",
		},
	}

	return e.createAndWaitJob(ctx, job.Namespace, jobName, job.Name, image, envVars, volumes, volumeMounts)
}

func (e *Executor) executeS3(ctx context.Context, job *backupjob.BackupJob, run *backupjob.BackupRun) error {
	if job.S3 == nil {
		return fmt.Errorf("s3 config is required for s3 backup job")
	}

	repoParts := strings.SplitN(job.RepositoryRef, "/", 2)
	if len(repoParts) != 2 {
		return fmt.Errorf("invalid repositoryRef format: %s", job.RepositoryRef)
	}

	repo, err := e.repoProvider.Get(ctx, repoParts[0], repoParts[1])
	if err != nil {
		return fmt.Errorf("resolving repository %s: %w", job.RepositoryRef, err)
	}

	image := job.Image
	if image == "" {
		image = e.options.S3BackupImage
	}

	timestamp := time.Now().Format("20060102-150405")
	jobName := fmt.Sprintf("cryo-s3-%s-%s", job.Name, timestamp)
	run.Name = jobName

	envVars := repoSecretEnvVars(repo)
	envVars = append(envVars,
		corev1.EnvVar{Name: "S3_ENDPOINT", Value: job.S3.Endpoint},
		corev1.EnvVar{Name: "S3_BUCKET", Value: job.S3.Bucket},
	)

	if job.S3.CredentialsSecretRef != nil {
		envVars = append(envVars,
			corev1.EnvVar{
				Name: "S3_ACCESS_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: job.S3.CredentialsSecretRef.Name},
						Key:                  job.S3.CredentialsSecretRef.AccessKeyKey,
					},
				},
			},
			corev1.EnvVar{
				Name: "S3_SECRET_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: job.S3.CredentialsSecretRef.Name},
						Key:                  job.S3.CredentialsSecretRef.SecretKeyKey,
					},
				},
			},
		)
	}

	envVars = append(envVars, retentionEnvVars(job.Retention)...)

	// Staging PVC
	stagingSize := "1Gi"
	stagingStorageClass := ""
	if job.S3.StagingSize != "" {
		stagingSize = job.S3.StagingSize
	}
	if job.S3.StagingStorageClassName != "" {
		stagingStorageClass = job.S3.StagingStorageClassName
	} else if e.settingsProvider != nil {
		s, err := e.settingsProvider.Get(ctx)
		if err == nil && s.DefaultStorageClassName != "" {
			stagingStorageClass = s.DefaultStorageClassName
		}
	}

	stagingPVCName := jobName + "-staging"
	if err := e.createStagingPVC(ctx, job.Namespace, stagingPVCName, stagingSize, stagingStorageClass); err != nil {
		return fmt.Errorf("creating staging PVC: %w", err)
	}
	defer e.cleanupStagingPVC(ctx, job.Namespace, stagingPVCName)

	envVars = append(envVars, corev1.EnvVar{Name: "TEMP_DIR", Value: "/staging/s3-backup"})

	volumes := []corev1.Volume{
		{
			Name: "staging",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: stagingPVCName,
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "staging",
			MountPath: "/staging",
		},
	}

	return e.createAndWaitJob(ctx, job.Namespace, jobName, job.Name, image, envVars, volumes, volumeMounts)
}

func (e *Executor) executePVC(ctx context.Context, job *backupjob.BackupJob, run *backupjob.BackupRun) error {
	if job.PVC == nil {
		return fmt.Errorf("pvc config is required for pvc backup job")
	}

	timestamp := time.Now().Format("20060102-150405")
	jobName := fmt.Sprintf("cryo-pvc-%s-%s", job.Name, timestamp)
	run.Name = jobName

	return e.orchestrator.Run(ctx, job, jobName)
}

func (e *Executor) createAndWaitJob(ctx context.Context, namespace, jobName, backupJobName, image string, envVars []corev1.EnvVar, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) error {
	var backoffLimit int32 = 0
	ttl := int32(604800) // 7 days default
	if e.settingsProvider != nil {
		if s, err := e.settingsProvider.Get(ctx); err == nil && s.JobTTLSeconds > 0 {
			ttl = s.JobTTLSeconds
		}
	}

	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"go-cryo.github.com/backup-job":           backupJobName,
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
						},
					},
				},
			},
		},
	}

	if len(volumes) > 0 {
		k8sJob.Spec.Template.Spec.Volumes = volumes
	}
	if len(volumeMounts) > 0 {
		k8sJob.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts
	}

	created, err := e.clientSet.BatchV1().Jobs(namespace).Create(ctx, k8sJob, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating kubernetes job %s/%s: %w", namespace, jobName, err)
	}

	log.Info().Str("job", created.Namespace+"/"+created.Name).Msg("created kubernetes backup job")

	err = e.waitForJobCompletion(ctx, namespace, jobName)
	e.cleanupJobPods(ctx, namespace, jobName)
	return err
}

func (e *Executor) waitForJobCompletion(ctx context.Context, namespace, jobName string) error {
	watcher, err := e.clientSet.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
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

func (e *Executor) cleanupJobPods(ctx context.Context, namespace, jobName string) {
	labelSelector := fmt.Sprintf("job-name=%s", jobName)
	err := e.clientSet.CoreV1().Pods(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Warn().Err(err).Str("job", jobName).Msg("failed to cleanup job pods")
	} else {
		log.Debug().Str("job", jobName).Msg("cleaned up job pods")
	}
}

func (e *Executor) createStagingPVC(ctx context.Context, namespace, name, size, storageClass string) error {
	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return fmt.Errorf("parsing staging size %q: %w", size, err)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"go-cryo.github.com/staging": "true",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
		},
	}

	if storageClass != "" {
		pvc.Spec.StorageClassName = &storageClass
	}

	_, err = e.clientSet.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating staging PVC %s/%s: %w", namespace, name, err)
	}

	log.Info().Str("pvc", name).Str("size", size).Msg("created staging PVC")
	return nil
}

func (e *Executor) cleanupStagingPVC(ctx context.Context, namespace, name string) {
	err := e.clientSet.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Warn().Err(err).Str("pvc", name).Msg("failed to cleanup staging PVC")
	} else {
		log.Info().Str("pvc", name).Msg("cleaned up staging PVC")
	}
}

func repoSecretEnvVars(repo *repository.Repository) []corev1.EnvVar {
	optional := true

	envVars := []corev1.EnvVar{
		{Name: "RESTIC_REPOSITORY", Value: repo.URL},
		{
			Name: "RESTIC_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: repo.Name},
					Key:                  "RESTIC_PASSWORD",
				},
			},
		},
	}

	// AWS credentials: from host secret (modern) or repo secret (legacy)
	awsSecretName := repo.Name
	if repo.HostSecretName != "" {
		awsSecretName = repo.HostSecretName
	}

	envVars = append(envVars,
		corev1.EnvVar{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: awsSecretName},
					Key:                  "AWS_ACCESS_KEY_ID",
					Optional:             &optional,
				},
			},
		},
		corev1.EnvVar{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: awsSecretName},
					Key:                  "AWS_SECRET_ACCESS_KEY",
					Optional:             &optional,
				},
			},
		},
		corev1.EnvVar{
			Name: "AWS_DEFAULT_REGION",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: awsSecretName},
					Key:                  "AWS_DEFAULT_REGION",
					Optional:             &optional,
				},
			},
		},
	)

	return envVars
}

func retentionEnvVars(retention *backupjob.RetentionPolicy) []corev1.EnvVar {
	if retention == nil {
		return nil
	}

	var envVars []corev1.EnvVar
	if retention.KeepLast > 0 {
		envVars = append(envVars, corev1.EnvVar{Name: "KEEP_LAST", Value: strconv.Itoa(retention.KeepLast)})
	}
	if retention.KeepDaily > 0 {
		envVars = append(envVars, corev1.EnvVar{Name: "KEEP_DAILY", Value: strconv.Itoa(retention.KeepDaily)})
	}
	if retention.KeepWeekly > 0 {
		envVars = append(envVars, corev1.EnvVar{Name: "KEEP_WEEKLY", Value: strconv.Itoa(retention.KeepWeekly)})
	}
	if retention.KeepMonthly > 0 {
		envVars = append(envVars, corev1.EnvVar{Name: "KEEP_MONTHLY", Value: strconv.Itoa(retention.KeepMonthly)})
	}
	return envVars
}
