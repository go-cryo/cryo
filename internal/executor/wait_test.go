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
