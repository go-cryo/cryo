package executor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func jobWithCondition(condType batchv1.JobConditionType, msg string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "run", Namespace: "ops"},
		Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{
			{Type: condType, Status: corev1.ConditionTrue, Message: msg},
		}},
	}
}

func TestWaitForJob(t *testing.T) {
	jobPollInterval = time.Millisecond

	t.Run("complete", func(t *testing.T) {
		cs := fake.NewSimpleClientset(jobWithCondition(batchv1.JobComplete, ""))
		if err := waitForJob(context.Background(), cs, "ops", "run"); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	t.Run("failed", func(t *testing.T) {
		cs := fake.NewSimpleClientset(jobWithCondition(batchv1.JobFailed, "BackoffLimitExceeded"))
		err := waitForJob(context.Background(), cs, "ops", "run")
		if err == nil || !strings.Contains(err.Error(), "BackoffLimitExceeded") {
			t.Fatalf("want failure message, got %v", err)
		}
	})

	t.Run("deleted", func(t *testing.T) {
		cs := fake.NewSimpleClientset()
		if err := waitForJob(context.Background(), cs, "ops", "run"); err == nil {
			t.Fatal("want error for a vanished job")
		}
	})

	t.Run("survives transient errors", func(t *testing.T) {
		cs := fake.NewSimpleClientset(jobWithCondition(batchv1.JobComplete, ""))
		failures := 3
		cs.PrependReactor("get", "jobs", func(k8stesting.Action) (bool, runtime.Object, error) {
			if failures > 0 {
				failures--
				return true, nil, errors.New("net/http: TLS handshake timeout")
			}
			return false, nil, nil
		})
		if err := waitForJob(context.Background(), cs, "ops", "run"); err != nil {
			t.Fatalf("want nil after transient errors, got %v", err)
		}
	})
}

func failedPod(name string, created time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "ops",
			Labels:            map[string]string{"job-name": "run"},
			CreationTimestamp: metav1.NewTime(created),
		},
		Status: corev1.PodStatus{Phase: corev1.PodFailed},
	}
}

func TestRunWorkerJobFailure(t *testing.T) {
	jobPollInterval = time.Millisecond

	// The fake clientset has no Job controller: every created Job fails at once.
	failOnCreate := func(cs *fake.Clientset) {
		cs.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
			job := action.(k8stesting.CreateAction).GetObject().(*batchv1.Job)
			job.Status.Conditions = []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Message: "BackoffLimitExceeded"},
			}
			return false, nil, nil
		})
	}

	t.Run("error carries logs of the newest failed pod", func(t *testing.T) {
		now := time.Now()
		cs := fake.NewSimpleClientset(failedPod("run-first", now.Add(-time.Minute)), failedPod("run-retry", now))
		failOnCreate(cs)

		err := runWorkerJob(context.Background(), cs, nil, "ops", "run", "nightly", "img", nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "BackoffLimitExceeded") ||
			!strings.Contains(err.Error(), "pod run-retry") || !strings.Contains(err.Error(), "fake logs") {
			t.Fatalf("want job failure with the retry pod's logs, got %v", err)
		}

		job, getErr := cs.BatchV1().Jobs("ops").Get(context.Background(), "run", metav1.GetOptions{})
		if getErr != nil {
			t.Fatal(getErr)
		}
		if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 1 {
			t.Errorf("BackoffLimit = %v, want 1", job.Spec.BackoffLimit)
		}
	})

	t.Run("log capture failure keeps the original error", func(t *testing.T) {
		cs := fake.NewSimpleClientset()
		failOnCreate(cs)

		err := runWorkerJob(context.Background(), cs, nil, "ops", "run", "nightly", "img", nil, nil, nil)
		if err == nil || err.Error() != "job ops/run failed: BackoffLimitExceeded" {
			t.Fatalf("want the unchanged job error, got %v", err)
		}
	})
}

func TestTailBytes(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"aaaa\nbbbb\ncc", 6, "cc"},
		{"aaaa\nbbbb\ncc", 7, "bbbb\ncc"},
		{"aaaa\nbbbb\ncc", 8, "bbbb\ncc"},
		{"abcdefghij", 4, "ghij"},
	}
	for _, c := range cases {
		if got := tailBytes(c.in, c.n); got != c.want {
			t.Errorf("tailBytes(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
