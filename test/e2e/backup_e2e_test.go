//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/go-cryo/cryo/internal/repositoryhost"
)

func TestE2E_BackupExecution_PSQL(t *testing.T) {
	hostName := "e2e-exec-psql-host"
	repoName := "e2e-exec-psql-repo"
	jobName := "e2e-exec-psql-job"

	// Create host pointing to MinIO
	createHost(t, hostName)
	defer deleteResource(t, "hosts", testNamespace, hostName)

	// Create repository
	createRepo(t, repoName, hostName, "psql-e2e")
	defer deleteResource(t, "repositories", testNamespace, repoName)

	// Create PSQL backup job
	jobReq := backupjob.CreateBackupJobRequest{
		Name:          jobName,
		Namespace:     testNamespace,
		Type:          backupjob.BackupJobTypePSQL,
		Schedule:      "0 0 1 1 *", // yearly, won't trigger during test
		RepositoryRef: testNamespace + "/" + repoName,
		PSQL: &backupjob.PSQLConfig{
			Hostname: "postgres",
			Port:     5432,
			Username: "testuser",
			Password: "testpass",
			Database: "testdb",
		},
	}
	body, _ := json.Marshal(jobReq)
	resp, err := authClient.Post(serverURL+"/api/v1/backupjobs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("creating backup job: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
	defer deleteResource(t, "backupjobs", testNamespace, jobName)

	// Trigger backup
	triggerURL := fmt.Sprintf("%s/api/v1/backupjobs/%s/%s/trigger", serverURL, testNamespace, jobName)
	resp, err = authClient.Post(triggerURL, "", nil)
	if err != nil {
		t.Fatalf("triggering backup: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for trigger, got %d", resp.StatusCode)
	}

	// Wait for run to succeed
	t.Log("Waiting for PSQL backup to complete...")
	waitForBackupRunSucceeded(t, testNamespace, jobName, 5*time.Minute)
	t.Log("PSQL backup succeeded")
}

func TestE2E_BackupExecution_S3(t *testing.T) {
	hostName := "e2e-exec-s3-host"
	repoName := "e2e-exec-s3-repo"
	jobName := "e2e-exec-s3-job"

	// Create host pointing to MinIO
	createHost(t, hostName)
	defer deleteResource(t, "hosts", testNamespace, hostName)

	// Create repository
	createRepo(t, repoName, hostName, "s3-e2e")
	defer deleteResource(t, "repositories", testNamespace, repoName)

	// Create S3 backup job (AccessKey/SecretKey auto-create credential secret)
	jobReq := backupjob.CreateBackupJobRequest{
		Name:          jobName,
		Namespace:     testNamespace,
		Type:          backupjob.BackupJobTypeS3,
		Schedule:      "0 0 1 1 *",
		RepositoryRef: testNamespace + "/" + repoName,
		S3: &backupjob.S3Config{
			Endpoint:  "http://rustfs:9000",
			Bucket:    "test-source",
			AccessKey: "rustfsadmin",
			SecretKey: "rustfsadmin",
		},
	}
	body, _ := json.Marshal(jobReq)
	resp, err := authClient.Post(serverURL+"/api/v1/backupjobs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("creating backup job: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
	defer deleteResource(t, "backupjobs", testNamespace, jobName)

	// Trigger backup
	triggerURL := fmt.Sprintf("%s/api/v1/backupjobs/%s/%s/trigger", serverURL, testNamespace, jobName)
	resp, err = authClient.Post(triggerURL, "", nil)
	if err != nil {
		t.Fatalf("triggering backup: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for trigger, got %d", resp.StatusCode)
	}

	// Wait for run to succeed
	t.Log("Waiting for S3 backup to complete...")
	waitForBackupRunSucceeded(t, testNamespace, jobName, 5*time.Minute)
	t.Log("S3 backup succeeded")
}

func TestE2E_BackupExecution_PVC(t *testing.T) {
	hostName := "e2e-exec-pvc-host"
	repoName := "e2e-exec-pvc-repo"
	jobName := "e2e-exec-pvc-job"

	// Create host pointing to MinIO
	createHost(t, hostName)
	defer deleteResource(t, "hosts", testNamespace, hostName)

	// Create repository
	createRepo(t, repoName, hostName, "pvc-e2e")
	defer deleteResource(t, "repositories", testNamespace, repoName)

	// Create PVC backup job
	jobReq := backupjob.CreateBackupJobRequest{
		Name:          jobName,
		Namespace:     testNamespace,
		Type:          backupjob.BackupJobTypePVC,
		Schedule:      "0 0 1 1 *",
		RepositoryRef: testNamespace + "/" + repoName,
		PVC: &backupjob.PVCConfig{
			ClaimName:               "test-data",
			VolumeSnapshotClassName: "csi-hostpath-snapclass",
			SnapshotRetention:       3,
		},
	}
	body, _ := json.Marshal(jobReq)
	resp, err := authClient.Post(serverURL+"/api/v1/backupjobs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("creating backup job: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
	defer deleteResource(t, "backupjobs", testNamespace, jobName)

	// Trigger backup
	triggerURL := fmt.Sprintf("%s/api/v1/backupjobs/%s/%s/trigger", serverURL, testNamespace, jobName)
	resp, err = authClient.Post(triggerURL, "", nil)
	if err != nil {
		t.Fatalf("triggering backup: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for trigger, got %d", resp.StatusCode)
	}

	// PVC backup is slower due to snapshot creation
	t.Log("Waiting for PVC backup to complete (snapshot creation may take time)...")
	waitForBackupRunSucceeded(t, testNamespace, jobName, 8*time.Minute)
	t.Log("PVC backup succeeded")
}

// createHost creates a repository host pointing to MinIO via the API.
func createHost(t testing.TB, name string) {
	t.Helper()
	hostReq := repositoryhost.CreateHostRequest{
		Name:               name,
		Namespace:          testNamespace,
		BaseURL:            "s3:http://rustfs:9000/cryo-repo",
		AwsAccessKeyID:     "rustfsadmin",
		AwsSecretAccessKey: "rustfsadmin",
	}
	body, _ := json.Marshal(hostReq)
	resp, err := authClient.Post(serverURL+"/api/v1/hosts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("creating host %s: %v", name, err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for host %s, got %d: %s", name, resp.StatusCode, string(respBody))
	}
}

// createRepo creates a repository referencing a host via the API.
func createRepo(t testing.TB, name, hostName, path string) {
	t.Helper()
	repoReq := repository.CreateRepositoryRequest{
		Name:           name,
		Namespace:      testNamespace,
		HostRef:        testNamespace + "/" + hostName,
		Path:           path,
		ResticPassword: "e2e-test-password",
	}
	body, _ := json.Marshal(repoReq)
	resp, err := authClient.Post(serverURL+"/api/v1/repositories", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("creating repository %s: %v", name, err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for repo %s, got %d: %s", name, resp.StatusCode, string(respBody))
	}
}
