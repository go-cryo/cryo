package executor

import (
	"testing"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRunStore_SetAndGetActive(t *testing.T) {
	store := NewRunStore(fake.NewSimpleClientset())

	run := &backupjob.BackupRun{
		JobName:   "my-backup",
		Namespace: "default",
		Name:      "my-backup-12345",
		Status:    backupjob.BackupRunStatusRunning,
	}

	store.SetActive(run)
	got := store.GetActive("default", "my-backup")
	if got == nil {
		t.Fatal("expected active run, got nil")
	}
	if got.Name != "my-backup-12345" {
		t.Errorf("Name = %q, want %q", got.Name, "my-backup-12345")
	}
	if got.Status != backupjob.BackupRunStatusRunning {
		t.Errorf("Status = %q, want %q", got.Status, backupjob.BackupRunStatusRunning)
	}
}

func TestRunStore_GetActiveNotFound(t *testing.T) {
	store := NewRunStore(fake.NewSimpleClientset())

	got := store.GetActive("default", "nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent active run, got %v", got)
	}
}

func TestRunStore_ClearActive(t *testing.T) {
	store := NewRunStore(fake.NewSimpleClientset())

	run := &backupjob.BackupRun{
		JobName:   "my-backup",
		Namespace: "default",
		Name:      "my-backup-12345",
		Status:    backupjob.BackupRunStatusRunning,
	}

	store.SetActive(run)
	store.ClearActive("default", "my-backup")

	got := store.GetActive("default", "my-backup")
	if got != nil {
		t.Errorf("expected nil after ClearActive, got %v", got)
	}
}

func TestRunStore_ClearActiveNonexistent(t *testing.T) {
	store := NewRunStore(fake.NewSimpleClientset())

	// Should not panic
	store.ClearActive("default", "nonexistent")
}

func TestRunStore_OverwriteActive(t *testing.T) {
	store := NewRunStore(fake.NewSimpleClientset())

	run1 := &backupjob.BackupRun{
		JobName:   "my-backup",
		Namespace: "default",
		Name:      "my-backup-11111",
		Status:    backupjob.BackupRunStatusRunning,
	}
	run2 := &backupjob.BackupRun{
		JobName:   "my-backup",
		Namespace: "default",
		Name:      "my-backup-22222",
		Status:    backupjob.BackupRunStatusRunning,
	}

	store.SetActive(run1)
	store.SetActive(run2)

	got := store.GetActive("default", "my-backup")
	if got == nil {
		t.Fatal("expected active run, got nil")
	}
	if got.Name != "my-backup-22222" {
		t.Errorf("Name = %q, want %q (should be overwritten)", got.Name, "my-backup-22222")
	}
}

func TestJobToBackupRun_Succeeded(t *testing.T) {
	now := time.Now()
	startTime := metav1.NewTime(now.Add(-5 * time.Minute))
	endTime := metav1.NewTime(now)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-job-12345",
			Namespace: "default",
			Labels: map[string]string{
				"go-cryo.github.com/backup-job": "my-backup",
			},
		},
		Status: batchv1.JobStatus{
			StartTime: &startTime,
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobComplete,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: endTime,
				},
			},
		},
	}

	run := jobToBackupRun(job)
	if run.Status != backupjob.BackupRunStatusSucceeded {
		t.Errorf("Status = %q, want %q", run.Status, backupjob.BackupRunStatusSucceeded)
	}
	if run.JobName != "my-backup" {
		t.Errorf("JobName = %q, want %q", run.JobName, "my-backup")
	}
	if run.Namespace != "default" {
		t.Errorf("Namespace = %q, want %q", run.Namespace, "default")
	}
	if run.Name != "backup-job-12345" {
		t.Errorf("Name = %q, want %q", run.Name, "backup-job-12345")
	}
	if run.StartTime == nil {
		t.Fatal("StartTime should not be nil")
	}
	if run.EndTime == nil {
		t.Fatal("EndTime should not be nil")
	}
}

func TestJobToBackupRun_Failed(t *testing.T) {
	now := time.Now()
	startTime := metav1.NewTime(now.Add(-5 * time.Minute))
	endTime := metav1.NewTime(now)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-job-12345",
			Namespace: "default",
			Labels: map[string]string{
				"go-cryo.github.com/backup-job": "my-backup",
			},
		},
		Status: batchv1.JobStatus{
			StartTime: &startTime,
			Conditions: []batchv1.JobCondition{
				{
					Type:               batchv1.JobFailed,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: endTime,
					Message:            "BackoffLimitExceeded",
				},
			},
		},
	}

	run := jobToBackupRun(job)
	if run.Status != backupjob.BackupRunStatusFailed {
		t.Errorf("Status = %q, want %q", run.Status, backupjob.BackupRunStatusFailed)
	}
	if run.Message != "BackoffLimitExceeded" {
		t.Errorf("Message = %q, want %q", run.Message, "BackoffLimitExceeded")
	}
	if run.EndTime == nil {
		t.Fatal("EndTime should not be nil for failed job")
	}
}

func TestJobToBackupRun_Running(t *testing.T) {
	now := time.Now()
	startTime := metav1.NewTime(now.Add(-1 * time.Minute))

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-job-12345",
			Namespace: "default",
			Labels: map[string]string{
				"go-cryo.github.com/backup-job": "my-backup",
			},
		},
		Status: batchv1.JobStatus{
			StartTime: &startTime,
		},
	}

	run := jobToBackupRun(job)
	if run.Status != backupjob.BackupRunStatusRunning {
		t.Errorf("Status = %q, want %q", run.Status, backupjob.BackupRunStatusRunning)
	}
	if run.StartTime == nil {
		t.Fatal("StartTime should not be nil")
	}
	if run.EndTime != nil {
		t.Errorf("EndTime should be nil for running job, got %v", run.EndTime)
	}
}

func TestJobToBackupRun_NoStartTime(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-job-12345",
			Namespace: "default",
			Labels: map[string]string{
				"go-cryo.github.com/backup-job": "my-backup",
			},
		},
		Status: batchv1.JobStatus{},
	}

	run := jobToBackupRun(job)
	if run.Status != backupjob.BackupRunStatusRunning {
		t.Errorf("Status = %q, want %q", run.Status, backupjob.BackupRunStatusRunning)
	}
	if run.StartTime != nil {
		t.Errorf("StartTime should be nil, got %v", run.StartTime)
	}
}
