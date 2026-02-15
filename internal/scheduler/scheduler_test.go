package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
)

// --- mock executor ---

type mockExecutor struct {
	mu       sync.Mutex
	executed []*backupjob.BackupJob
}

func (m *mockExecutor) Execute(_ context.Context, job *backupjob.BackupJob) (*backupjob.BackupRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executed = append(m.executed, job)
	return &backupjob.BackupRun{
		JobName:   job.Name,
		Namespace: job.Namespace,
		Name:      "run-1",
		Status:    backupjob.BackupRunStatusSucceeded,
	}, nil
}

func (m *mockExecutor) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.executed)
}

// --- mock provider ---

type mockProvider struct {
	mu   sync.Mutex
	jobs map[string]*backupjob.BackupJob
}

func newMockProvider() *mockProvider {
	return &mockProvider{jobs: make(map[string]*backupjob.BackupJob)}
}

func (m *mockProvider) addJob(job *backupjob.BackupJob) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.Namespace+"/"+job.Name] = job
}

func (m *mockProvider) List(_ context.Context) ([]*backupjob.BackupJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*backupjob.BackupJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		result = append(result, j)
	}
	return result, nil
}

func (m *mockProvider) Get(_ context.Context, namespace, name string) (*backupjob.BackupJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := namespace + "/" + name
	job, ok := m.jobs[key]
	if !ok {
		return nil, fmt.Errorf("backup job %s not found", key)
	}
	return job, nil
}

func (m *mockProvider) Create(_ context.Context, _ *backupjob.CreateBackupJobRequest) (*backupjob.BackupJob, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) Update(_ context.Context, _, _ string, _ *backupjob.UpdateBackupJobRequest) (*backupjob.BackupJob, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) Delete(_ context.Context, _, _ string) error {
	return fmt.Errorf("not implemented")
}

// --- tests ---

func TestSyncAll_SchedulesNonSuspendedJobs(t *testing.T) {
	provider := newMockProvider()
	provider.addJob(&backupjob.BackupJob{
		Name: "active-job", Namespace: "default",
		Schedule: "*/5 * * * *", Suspend: false,
	})
	provider.addJob(&backupjob.BackupJob{
		Name: "suspended-job", Namespace: "default",
		Schedule: "*/5 * * * *", Suspend: true,
	})

	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	if err := s.SyncAll(context.Background()); err != nil {
		t.Fatalf("SyncAll error: %v", err)
	}

	if next := s.NextRun("default", "active-job"); next == nil {
		t.Error("expected NextRun for active-job, got nil")
	}
	if next := s.NextRun("default", "suspended-job"); next != nil {
		t.Errorf("expected nil NextRun for suspended-job, got %v", next)
	}
}

func TestSyncAll_SkipsSuspended(t *testing.T) {
	provider := newMockProvider()
	provider.addJob(&backupjob.BackupJob{
		Name: "job-a", Namespace: "ns1",
		Schedule: "0 * * * *", Suspend: true,
	})
	provider.addJob(&backupjob.BackupJob{
		Name: "job-b", Namespace: "ns1",
		Schedule: "0 * * * *", Suspend: true,
	})

	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	if err := s.SyncAll(context.Background()); err != nil {
		t.Fatalf("SyncAll error: %v", err)
	}

	if next := s.NextRun("ns1", "job-a"); next != nil {
		t.Errorf("expected nil for suspended job-a, got %v", next)
	}
	if next := s.NextRun("ns1", "job-b"); next != nil {
		t.Errorf("expected nil for suspended job-b, got %v", next)
	}
}

func TestSyncJob_AddsNewJob(t *testing.T) {
	provider := newMockProvider()
	provider.addJob(&backupjob.BackupJob{
		Name: "new-job", Namespace: "default",
		Schedule: "*/10 * * * *", Suspend: false,
	})

	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	s.SyncJob(context.Background(), "default", "new-job")

	if next := s.NextRun("default", "new-job"); next == nil {
		t.Error("expected NextRun for new-job after SyncJob, got nil")
	}
}

func TestSyncJob_UpdatesExistingJob(t *testing.T) {
	provider := newMockProvider()
	provider.addJob(&backupjob.BackupJob{
		Name: "update-job", Namespace: "default",
		Schedule: "*/5 * * * *", Suspend: false,
	})

	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	s.SyncJob(context.Background(), "default", "update-job")
	next1 := s.NextRun("default", "update-job")
	if next1 == nil {
		t.Fatal("expected NextRun after first SyncJob, got nil")
	}

	// Update schedule and sync again
	provider.addJob(&backupjob.BackupJob{
		Name: "update-job", Namespace: "default",
		Schedule: "*/15 * * * *", Suspend: false,
	})
	s.SyncJob(context.Background(), "default", "update-job")

	next2 := s.NextRun("default", "update-job")
	if next2 == nil {
		t.Fatal("expected NextRun after second SyncJob, got nil")
	}
}

func TestSyncJob_RemovesSuspendedJob(t *testing.T) {
	provider := newMockProvider()
	provider.addJob(&backupjob.BackupJob{
		Name: "toggle-job", Namespace: "default",
		Schedule: "*/5 * * * *", Suspend: false,
	})

	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	s.SyncJob(context.Background(), "default", "toggle-job")
	if next := s.NextRun("default", "toggle-job"); next == nil {
		t.Fatal("expected NextRun after scheduling, got nil")
	}

	// Suspend the job
	provider.addJob(&backupjob.BackupJob{
		Name: "toggle-job", Namespace: "default",
		Schedule: "*/5 * * * *", Suspend: true,
	})
	s.SyncJob(context.Background(), "default", "toggle-job")

	if next := s.NextRun("default", "toggle-job"); next != nil {
		t.Errorf("expected nil NextRun after suspending, got %v", next)
	}
}

func TestRemoveJob_RemovesScheduled(t *testing.T) {
	provider := newMockProvider()
	provider.addJob(&backupjob.BackupJob{
		Name: "remove-me", Namespace: "default",
		Schedule: "*/5 * * * *", Suspend: false,
	})

	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	s.SyncJob(context.Background(), "default", "remove-me")
	if next := s.NextRun("default", "remove-me"); next == nil {
		t.Fatal("expected NextRun after scheduling, got nil")
	}

	s.RemoveJob("default", "remove-me")
	if next := s.NextRun("default", "remove-me"); next != nil {
		t.Errorf("expected nil NextRun after removal, got %v", next)
	}
}

func TestRemoveJob_NoOpForUnknown(t *testing.T) {
	provider := newMockProvider()
	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	// Should not panic
	s.RemoveJob("default", "nonexistent")
}

func TestTriggerNow_Success(t *testing.T) {
	provider := newMockProvider()
	provider.addJob(&backupjob.BackupJob{
		Name: "trigger-job", Namespace: "default",
		Schedule: "0 0 31 2 *", Suspend: false,
	})

	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)

	err := s.TriggerNow(context.Background(), "default", "trigger-job")
	if err != nil {
		t.Fatalf("TriggerNow error: %v", err)
	}

	// Wait for the goroutine to execute
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if exec.count() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if exec.count() != 1 {
		t.Fatalf("expected executor to be called once, got %d", exec.count())
	}
}

func TestTriggerNow_UnknownJob(t *testing.T) {
	provider := newMockProvider()
	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)

	err := s.TriggerNow(context.Background(), "default", "no-such-job")
	if err == nil {
		t.Error("expected error for unknown job, got nil")
	}
}

func TestNextRun_Scheduled(t *testing.T) {
	provider := newMockProvider()
	provider.addJob(&backupjob.BackupJob{
		Name: "cron-job", Namespace: "default",
		Schedule: "*/5 * * * *", Suspend: false,
	})

	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	s.SyncJob(context.Background(), "default", "cron-job")

	next := s.NextRun("default", "cron-job")
	if next == nil {
		t.Fatal("expected NextRun for scheduled job, got nil")
	}
	if !next.After(time.Now()) {
		t.Errorf("expected NextRun to be in the future, got %v", next)
	}
}

func TestNextRun_Unknown(t *testing.T) {
	provider := newMockProvider()
	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	defer s.Stop()

	if next := s.NextRun("default", "unknown"); next != nil {
		t.Errorf("expected nil for unknown job, got %v", next)
	}
}

func TestStartStop(t *testing.T) {
	provider := newMockProvider()
	exec := &mockExecutor{}
	s := NewScheduler(provider, exec)
	s.Start()
	s.Stop()
}
