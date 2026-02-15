package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

type Executor interface {
	Execute(ctx context.Context, job *backupjob.BackupJob) (*backupjob.BackupRun, error)
}

type Scheduler struct {
	cron     *cron.Cron
	mu       sync.RWMutex
	entries  map[string]cron.EntryID
	provider backupjob.Provider
	executor Executor
}

func NewScheduler(provider backupjob.Provider, executor Executor) *Scheduler {
	return &Scheduler{
		cron:     cron.New(),
		entries:  make(map[string]cron.EntryID),
		provider: provider,
		executor: executor,
	}
}

func (s *Scheduler) SyncAll(ctx context.Context) error {
	jobs, err := s.provider.List(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	for key, entryID := range s.entries {
		s.cron.Remove(entryID)
		delete(s.entries, key)
	}
	s.mu.Unlock()

	for _, job := range jobs {
		if job.Suspend {
			log.Debug().Str("job", job.Namespace+"/"+job.Name).Msg("skipping suspended backup job")
			continue
		}
		s.scheduleJob(job)
	}

	return nil
}

func (s *Scheduler) SyncJob(ctx context.Context, namespace, name string) {
	key := namespace + "/" + name

	job, err := s.provider.Get(ctx, namespace, name)
	if err != nil {
		log.Warn().Err(err).Str("job", key).Msg("failed to get backup job for sync, removing schedule")
		s.RemoveJob(namespace, name)
		return
	}

	s.mu.Lock()
	if entryID, ok := s.entries[key]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, key)
	}
	s.mu.Unlock()

	if !job.Suspend {
		s.scheduleJob(job)
	}
}

func (s *Scheduler) RemoveJob(namespace, name string) {
	key := namespace + "/" + name
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entries[key]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, key)
		log.Debug().Str("job", key).Msg("removed backup job from scheduler")
	}
}

func (s *Scheduler) TriggerNow(ctx context.Context, namespace, name string) error {
	job, err := s.provider.Get(ctx, namespace, name)
	if err != nil {
		return err
	}

	go func() {
		_, err := s.executor.Execute(context.Background(), job)
		if err != nil {
			log.Error().Err(err).Str("job", namespace+"/"+name).Msg("manual trigger execution failed")
		}
	}()

	return nil
}

func (s *Scheduler) Start() {
	s.cron.Start()
	log.Info().Msg("backup job scheduler started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("backup job scheduler stopped")
}

func (s *Scheduler) NextRun(namespace, name string) *time.Time {
	key := namespace + "/" + name
	s.mu.RLock()
	defer s.mu.RUnlock()
	entryID, ok := s.entries[key]
	if !ok {
		return nil
	}
	entry := s.cron.Entry(entryID)
	if entry.ID == 0 {
		return nil
	}
	next := entry.Next
	if next.IsZero() {
		return nil
	}
	return &next
}

func (s *Scheduler) scheduleJob(job *backupjob.BackupJob) {
	key := job.Namespace + "/" + job.Name
	jobCopy := *job

	entryID, err := s.cron.AddFunc(job.Schedule, func() {
		log.Info().Str("job", key).Msg("executing scheduled backup job")
		_, err := s.executor.Execute(context.Background(), &jobCopy)
		if err != nil {
			log.Error().Err(err).Str("job", key).Msg("scheduled backup job execution failed")
		}
	})
	if err != nil {
		log.Error().Err(err).Str("job", key).Str("schedule", job.Schedule).Msg("failed to schedule backup job")
		return
	}

	s.mu.Lock()
	s.entries[key] = entryID
	s.mu.Unlock()

	log.Info().Str("job", key).Str("schedule", job.Schedule).Msg("scheduled backup job")
}
