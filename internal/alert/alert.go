package alert

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/executor"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const overdueCheckInterval = 15 * time.Minute

// Alerter mails on failed backup runs and on scheduled jobs whose last
// successful run is older than their latest due time plus a grace period.
// The overdue check catches what failure alerts cannot: runs that never
// started at all.
type Alerter struct {
	mailer       *Mailer
	prefix       string
	jobs         backupjob.Provider
	runs         *executor.RunStore
	overdueAfter time.Duration
	notified     map[string]time.Time // job key -> due time already alerted; checkOverdue only
}

func NewAlerter(mailer *Mailer, prefix string, jobs backupjob.Provider, runs *executor.RunStore, overdueAfter time.Duration) *Alerter {
	return &Alerter{
		mailer:       mailer,
		prefix:       prefix,
		jobs:         jobs,
		runs:         runs,
		overdueAfter: overdueAfter,
		notified:     make(map[string]time.Time),
	}
}

// RunFailed is the executor's failure hook; it covers scheduled and manual runs.
func (a *Alerter) RunFailed(job *backupjob.BackupJob, run *backupjob.BackupRun) {
	key := job.Namespace + "/" + job.Name
	var b strings.Builder
	fmt.Fprintf(&b, "Backup job %s failed.\n\n", key)
	fmt.Fprintf(&b, "Type:     %s\n", job.Type)
	fmt.Fprintf(&b, "Schedule: %s\n", job.Schedule)
	if run.Name != "" {
		fmt.Fprintf(&b, "Run:      %s\n", run.Name)
	}
	if run.StartTime != nil {
		fmt.Fprintf(&b, "Started:  %s\n", formatTime(*run.StartTime))
	}
	if run.EndTime != nil {
		fmt.Fprintf(&b, "Ended:    %s\n", formatTime(*run.EndTime))
	}
	fmt.Fprintf(&b, "\nError:\n%s\n", run.Message)

	a.send(fmt.Sprintf("%s backup failed: %s", a.prefix, key), b.String())
}

// Run checks for overdue jobs until ctx is cancelled. Alert state is in memory,
// so a controller restart re-sends one digest for jobs that are still overdue.
func (a *Alerter) Run(ctx context.Context) {
	ticker := time.NewTicker(overdueCheckInterval)
	defer ticker.Stop()
	for {
		a.checkOverdue(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Alerter) checkOverdue(ctx context.Context, now time.Time) {
	jobs, err := a.jobs.List(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("overdue check: listing backup jobs failed")
		return
	}

	var lines []string
	pending := make(map[string]time.Time)
	for _, job := range jobs {
		key := job.Namespace + "/" + job.Name
		if job.Suspend {
			delete(a.notified, key)
			continue
		}
		schedule, err := cron.ParseStandard(job.Schedule)
		if err != nil {
			continue
		}
		runs, err := a.runs.ListRuns(ctx, job.Namespace, job.Name)
		if err != nil {
			log.Warn().Err(err).Str("job", key).Msg("overdue check: listing runs failed")
			continue
		}
		lastSuccess := lastSuccessfulRun(runs)

		due, overdue := overdueSince(schedule, job.CreatedAt, lastSuccess, now, a.overdueAfter)
		if !overdue {
			delete(a.notified, key)
			continue
		}
		if a.notified[key].Equal(due) {
			continue
		}
		pending[key] = due

		last := "none in the retained run history"
		if !lastSuccess.IsZero() {
			last = formatTime(lastSuccess)
		}
		lines = append(lines, fmt.Sprintf("- %s (%s, schedule %q)\n  due:          %s\n  last success: %s", key, job.Type, job.Schedule, formatTime(due), last))
	}
	if len(lines) == 0 {
		return
	}

	sort.Strings(lines)
	body := fmt.Sprintf("%d backup job(s) have not completed a successful run for more than %s after their scheduled time:\n\n%s\n",
		len(lines), a.overdueAfter, strings.Join(lines, "\n\n"))
	// Only mark as alerted once the mail went out, so a failed send retries.
	if a.send(fmt.Sprintf("%s %d backup(s) overdue", a.prefix, len(lines)), body) {
		for key, due := range pending {
			a.notified[key] = due
		}
	}
}

// overdueSince reports whether the latest run due at least `after` ago has no
// successful run completed since, and returns that due time. Runs due before
// the job existed do not count.
//
// ponytail: success history comes from Kubernetes Jobs, which expire after the
// job TTL (default 7d); schedules longer than the TTL read as overdue. Read the
// latest restic snapshot instead if such schedules appear.
func overdueSince(schedule cron.Schedule, created, lastSuccess, now time.Time, after time.Duration) (time.Time, bool) {
	cutoff := now.Add(-after)
	start := cutoff.Add(-40 * 24 * time.Hour)
	if created.After(start) {
		start = created
	}

	var due time.Time
	for t := schedule.Next(start); !t.After(cutoff); t = schedule.Next(t) {
		due = t
	}
	if due.IsZero() {
		return due, false
	}
	return due, lastSuccess.Before(due)
}

func lastSuccessfulRun(runs []*backupjob.BackupRun) time.Time {
	var last time.Time
	for _, r := range runs {
		if r.Status == backupjob.BackupRunStatusSucceeded && r.EndTime != nil && r.EndTime.After(last) {
			last = *r.EndTime
		}
	}
	return last
}

func (a *Alerter) send(subject, body string) bool {
	if err := a.mailer.Send(subject, body); err != nil {
		log.Error().Err(err).Str("subject", subject).Msg("sending alert mail failed")
		return false
	}
	log.Info().Str("subject", subject).Msg("sent alert mail")
	return true
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 MST")
}
