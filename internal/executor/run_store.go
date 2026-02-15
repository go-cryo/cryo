package executor

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-cryo/cryo/internal/backupjob"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RunStore struct {
	mu        sync.RWMutex
	active    map[string]*backupjob.BackupRun
	clientSet kubernetes.Interface
}

func NewRunStore(clientSet kubernetes.Interface) *RunStore {
	return &RunStore{
		active:    make(map[string]*backupjob.BackupRun),
		clientSet: clientSet,
	}
}

func (s *RunStore) SetActive(run *backupjob.BackupRun) {
	key := run.Namespace + "/" + run.JobName
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[key] = run
}

func (s *RunStore) ClearActive(namespace, name string) {
	key := namespace + "/" + name
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, key)
}

func (s *RunStore) GetActive(namespace, name string) *backupjob.BackupRun {
	key := namespace + "/" + name
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active[key]
}

func (s *RunStore) ListRuns(ctx context.Context, namespace, name string) ([]*backupjob.BackupRun, error) {
	labelSelector := fmt.Sprintf("go-cryo.github.com/backup-job=%s,go-cryo.github.com/backup-job-namespace=%s", name, namespace)

	jobs, err := s.clientSet.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing jobs for backup job %s/%s: %w", namespace, name, err)
	}

	runs := make([]*backupjob.BackupRun, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		run := jobToBackupRun(&job)
		runs = append(runs, run)
	}

	return runs, nil
}

func jobToBackupRun(job *batchv1.Job) *backupjob.BackupRun {
	run := &backupjob.BackupRun{
		JobName:   job.Labels["go-cryo.github.com/backup-job"],
		Namespace: job.Namespace,
		Name:      job.Name,
	}

	if job.Status.StartTime != nil {
		t := job.Status.StartTime.Time
		run.StartTime = &t
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			run.Status = backupjob.BackupRunStatusSucceeded
			t := condition.LastTransitionTime.Time
			run.EndTime = &t
			return run
		}
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			run.Status = backupjob.BackupRunStatusFailed
			run.Message = condition.Message
			t := condition.LastTransitionTime.Time
			run.EndTime = &t
			return run
		}
	}

	run.Status = backupjob.BackupRunStatusRunning
	return run
}
