package alert

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/executor"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func at(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestOverdueSince(t *testing.T) {
	daily, _ := cron.ParseStandard("40 3 * * *")
	created := at("2026-09-01 00:00")
	grace := 6 * time.Hour

	tests := []struct {
		name        string
		created     time.Time
		lastSuccess time.Time
		now         time.Time
		wantOverdue bool
		wantDue     time.Time
	}{
		{"succeeded today", created, at("2026-09-23 03:47"), at("2026-09-23 12:00"), false, at("2026-09-23 03:40")},
		{"today's run missing, still in grace", created, at("2026-09-22 03:47"), at("2026-09-23 09:00"), false, at("2026-09-22 03:40")},
		{"today's run missing, grace passed", created, at("2026-09-22 03:47"), at("2026-09-23 09:41"), true, at("2026-09-23 03:40")},
		{"never succeeded", created, time.Time{}, at("2026-09-23 12:00"), true, at("2026-09-23 03:40")},
		{"job created after last due time", at("2026-09-23 08:00"), time.Time{}, at("2026-09-23 20:00"), false, time.Time{}},
		{"manual run after due time counts", created, at("2026-09-23 16:29"), at("2026-09-23 20:00"), false, at("2026-09-23 03:40")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			due, overdue := overdueSince(daily, tt.created, tt.lastSuccess, tt.now, grace)
			if overdue != tt.wantOverdue || !due.Equal(tt.wantDue) {
				t.Errorf("got (%s, %v), want (%s, %v)", due, overdue, tt.wantDue, tt.wantOverdue)
			}
		})
	}
}

// fakeSMTP is a minimal plaintext SMTP server that records each DATA payload.
type fakeSMTP struct {
	ln       net.Listener
	mu       sync.Mutex
	messages []string
	rcpts    []string
	reject   bool
}

func newFakeSMTP(t *testing.T) *fakeSMTP {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSMTP{ln: ln}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *fakeSMTP) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	reply := func(line string) { conn.Write([]byte(line + "\r\n")) }
	reply("220 fake")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			reply("250 fake")
		case strings.HasPrefix(cmd, "MAIL"):
			s.mu.Lock()
			reject := s.reject
			s.mu.Unlock()
			if reject {
				reply("451 try later")
				continue
			}
			reply("250 ok")
		case strings.HasPrefix(cmd, "RCPT"):
			s.mu.Lock()
			s.rcpts = append(s.rcpts, strings.TrimSpace(line[len("RCPT TO:"):]))
			s.mu.Unlock()
			reply("250 ok")
		case cmd == "DATA":
			reply("354 go")
			var data strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil || l == ".\r\n" {
					break
				}
				data.WriteString(l)
			}
			s.mu.Lock()
			s.messages = append(s.messages, data.String())
			s.mu.Unlock()
			reply("250 queued")
		case cmd == "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

func (s *fakeSMTP) mailer() *Mailer {
	return &Mailer{Host: "127.0.0.1", Port: s.ln.Addr().(*net.TCPAddr).Port, From: "cryo@example.org", To: []string{"a@example.org", "b@example.org"}}
}

func (s *fakeSMTP) sent() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.messages...)
}

func TestMailerSend(t *testing.T) {
	srv := newFakeSMTP(t)
	if err := srv.mailer().Send("[cryo] backup failed: ops/db", "Error:\nboom\n"); err != nil {
		t.Fatal(err)
	}
	msgs := srv.sent()
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	for _, want := range []string{"Subject: [cryo] backup failed: ops/db", "To: a@example.org, b@example.org", "boom"} {
		if !strings.Contains(msgs[0], want) {
			t.Errorf("message missing %q:\n%s", want, msgs[0])
		}
	}
	if len(srv.rcpts) != 2 {
		t.Errorf("got %d RCPT commands, want 2", len(srv.rcpts))
	}
}

func TestCheckOverdue_AlertsOnceAndRetriesFailedSends(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cnpg-intranet", Namespace: "ops",
			Labels:            map[string]string{"go-cryo.github.com/config": "true"},
			CreationTimestamp: metav1.NewTime(at("2026-09-01 00:00")),
		},
		Data: map[string]string{"config": "type: psql\nschedule: \"40 3 * * *\"\nrepositoryRef: ops/cnpg-intranet\npsql:\n  hostname: db\n  username: u\n  database: d\n  credentialSecretRef: s\n"},
	}
	// Last success two days ago: today's and yesterday's runs never happened.
	succeeded := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "run-1", Namespace: "ops", Labels: map[string]string{
			"go-cryo.github.com/backup-job": "cnpg-intranet", "go-cryo.github.com/backup-job-namespace": "ops",
		}},
		Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
			Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(at("2026-09-21 03:47")),
		}}},
	}
	cs := fake.NewSimpleClientset(cm, succeeded)

	srv := newFakeSMTP(t)
	a := NewAlerter(srv.mailer(), "[cryo]", backupjob.NewKubernetesProvider(cs, ""), executor.NewRunStore(cs), 6*time.Hour)
	now := at("2026-09-23 12:00")

	srv.reject = true
	a.checkOverdue(context.Background(), now)
	if len(srv.sent()) != 0 {
		t.Fatal("rejected mail should not count as sent")
	}

	srv.reject = false
	a.checkOverdue(context.Background(), now)
	a.checkOverdue(context.Background(), now.Add(15*time.Minute))
	msgs := srv.sent()
	if len(msgs) != 1 {
		t.Fatalf("got %d mails, want exactly 1 (retry after rejection, then no repeat)", len(msgs))
	}
	if !strings.Contains(msgs[0], "ops/cnpg-intranet") || !strings.Contains(msgs[0], "2026-09-21 03:47") {
		t.Errorf("digest missing job or last success:\n%s", msgs[0])
	}

	// The next day's due time is a new incident and alerts again.
	a.checkOverdue(context.Background(), now.Add(24*time.Hour))
	if len(srv.sent()) != 2 {
		t.Errorf("got %d mails, want a second alert for the next missed run", len(srv.sent()))
	}
}
