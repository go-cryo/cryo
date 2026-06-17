//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// applyManifest applies a YAML manifest file to the test namespace using kubectl.
func applyManifest(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", path, "-n", testNamespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply %s: %s: %w", path, string(output), err)
	}
	return nil
}

// waitForDeploymentReady polls until a Deployment has at least one available replica.
func waitForDeploymentReady(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for deployment %s/%s to be ready", testNamespace, name)
		case <-ticker.C:
			deploy, err := clientSet.AppsV1().Deployments(testNamespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			if deploy.Status.AvailableReplicas > 0 {
				return nil
			}
		}
	}
}

// waitForJobComplete polls until a K8s Job completes (succeeded or failed).
func waitForJobComplete(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			// Try to get pod logs for debugging
			pods, _ := clientSet.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: "job-name=" + name,
			})
			if pods != nil && len(pods.Items) > 0 {
				for _, pod := range pods.Items {
					for _, cs := range pod.Status.ContainerStatuses {
						if cs.State.Waiting != nil {
							return fmt.Errorf("timeout waiting for job %s: pod %s container %s waiting: %s (%s)",
								name, pod.Name, cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
						}
					}
				}
			}
			return fmt.Errorf("timeout waiting for job %s/%s to complete", testNamespace, name)
		case <-ticker.C:
			job, err := clientSet.BatchV1().Jobs(testNamespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			for _, condition := range job.Status.Conditions {
				if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
					return nil
				}
				if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
					return fmt.Errorf("job %s failed: %s", name, condition.Message)
				}
			}
		}
	}
}

// seedRustFS creates the buckets needed for testing (restic repository backend
// + S3 backup source) and uploads test data. The MinIO client (mc) speaks plain
// S3 and works against RustFS unchanged.
func seedRustFS(ctx context.Context) error {
	var backoffLimit int32 = 2
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rustfs-setup",
			Namespace: testNamespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "setup",
							Image:   "minio/mc:latest",
							Command: []string{"/bin/sh", "-c"},
							Args: []string{
								"until mc alias set rfs http://rustfs:9000 rustfsadmin rustfsadmin; do echo waiting for rustfs; sleep 2; done && " +
									"mc mb --ignore-existing rfs/cryo-repo && " +
									"mc mb --ignore-existing rfs/test-source && " +
									"echo 'test data for s3 backup' > /tmp/test-file.txt && " +
									"mc cp /tmp/test-file.txt rfs/test-source/test-file.txt",
							},
						},
					},
				},
			},
		},
	}

	_, err := clientSet.BatchV1().Jobs(testNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating rustfs setup job: %w", err)
	}
	return waitForJobComplete(ctx, "rustfs-setup", 3*time.Minute)
}

// seedPostgres creates test tables and data in PostgreSQL.
func seedPostgres(ctx context.Context) error {
	var backoffLimit int32 = 0
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-setup",
			Namespace: testNamespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "setup",
							Image: "postgres:16-alpine",
							Env: []corev1.EnvVar{
								{Name: "PGPASSWORD", Value: "testpass"},
							},
							Command: []string{"/bin/sh", "-c"},
							Args: []string{
								"psql -h postgres -U testuser -d testdb -c " +
									"\"CREATE TABLE IF NOT EXISTS e2e_test (id serial PRIMARY KEY, data text); " +
									"INSERT INTO e2e_test (data) VALUES ('row1'), ('row2'), ('row3');\"",
							},
						},
					},
				},
			},
		},
	}

	_, err := clientSet.BatchV1().Jobs(testNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating postgres setup job: %w", err)
	}
	return waitForJobComplete(ctx, "postgres-setup", 3*time.Minute)
}

// seedTestPVC writes test files into the test-data PVC.
func seedTestPVC(ctx context.Context) error {
	var backoffLimit int32 = 0
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-setup",
			Namespace: testNamespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "setup",
							Image: "alpine:3.20",
							Command: []string{"/bin/sh", "-c"},
							Args: []string{
								"mkdir -p /data/subdir && " +
									"echo 'test file 1' > /data/file1.txt && " +
									"echo 'test file 2' > /data/file2.txt && " +
									"echo 'nested file' > /data/subdir/nested.txt && " +
									"echo 'PVC seeded successfully'",
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/data"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "test-data",
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := clientSet.BatchV1().Jobs(testNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating pvc setup job: %w", err)
	}
	return waitForJobComplete(ctx, "pvc-setup", 3*time.Minute)
}

// waitForBackupRunSucceeded polls the runs API until a run reaches Succeeded status.
func waitForBackupRunSucceeded(t testing.TB, namespace, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for backup run %s/%s to succeed", namespace, name)
		}

		runsURL := fmt.Sprintf("%s/api/v1/backupjobs/%s/%s/runs", serverURL, namespace, name)
		resp, err := authClient.Get(runsURL)
		if err != nil {
			<-ticker.C
			continue
		}

		var runs []*backupjob.BackupRun
		json.NewDecoder(resp.Body).Decode(&runs)
		resp.Body.Close()

		for _, run := range runs {
			if run.Status == backupjob.BackupRunStatusSucceeded {
				return
			}
			if run.Status == backupjob.BackupRunStatusFailed {
				t.Fatalf("backup run %s failed: %s", run.Name, run.Message)
			}
		}

		<-ticker.C
	}
}

// deleteResource issues a DELETE request for a named API resource.
func deleteResource(t testing.TB, resource, namespace, name string) {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/%s/%s/%s", serverURL, resource, namespace, name)
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	authClient.Do(req)
}

// loginAdmin reads the bootstrapped admin credentials from the Kubernetes secret
// and logs in, returning an http.Client whose cookie jar carries the session.
func loginAdmin(ctx context.Context) (*http.Client, error) {
	secret, err := clientSet.CoreV1().Secrets(testNamespace).Get(ctx, adminSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting admin secret %q: %w", adminSecretName, err)
	}
	username := string(secret.Data["USERNAME"])
	password := string(secret.Data["PASSWORD"])
	if username == "" || password == "" {
		return nil, fmt.Errorf("admin secret %q missing USERNAME/PASSWORD", adminSecretName)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}
	client := &http.Client{Jar: jar}

	body, _ := json.Marshal(map[string]string{"identifier": username, "password": password})
	resp, err := client.Post(serverURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("posting login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login failed: status %d: %s", resp.StatusCode, string(respBody))
	}
	return client, nil
}

// envOrDefault returns the value of the named environment variable or the default value.
func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
