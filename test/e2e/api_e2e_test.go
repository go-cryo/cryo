//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/go-cryo/cryo/internal/repositoryhost"
	"github.com/go-cryo/cryo/internal/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestE2E_HealthAndVersion(t *testing.T) {
	t.Run("health returns 200", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/v1/health")
		if err != nil {
			t.Fatalf("GET /health failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]string
		json.NewDecoder(resp.Body).Decode(&body)
		if body["status"] != "ok" {
			t.Fatalf("expected status ok, got %q", body["status"])
		}
	})

	t.Run("version returns 200", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/v1/version")
		if err != nil {
			t.Fatalf("GET /version failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]string
		json.NewDecoder(resp.Body).Decode(&body)
		if body["version"] != "test" {
			t.Fatalf("expected version 'test', got %q", body["version"])
		}
	})
}

func TestE2E_HostCRUD(t *testing.T) {
	hostName := "e2e-test-host"

	// Create
	createReq := repositoryhost.CreateHostRequest{
		Name:      hostName,
		Namespace: testNamespace,
		BaseURL:   "s3:https://s3.example.com/bucket",
	}
	body, _ := json.Marshal(createReq)
	resp, err := http.Post(serverURL+"/api/v1/hosts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /hosts failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
	var createdHost repositoryhost.RepositoryHost
	json.NewDecoder(resp.Body).Decode(&createdHost)
	if createdHost.Name != hostName {
		t.Fatalf("expected name %q, got %q", hostName, createdHost.Name)
	}
	if createdHost.Type != repositoryhost.HostTypeS3 {
		t.Fatalf("expected type s3, got %q", createdHost.Type)
	}

	// List
	resp, err = http.Get(serverURL + "/api/v1/hosts")
	if err != nil {
		t.Fatalf("GET /hosts failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var hosts []*repositoryhost.RepositoryHost
	json.NewDecoder(resp.Body).Decode(&hosts)
	found := false
	for _, h := range hosts {
		if h.Name == hostName && h.Namespace == testNamespace {
			found = true
		}
	}
	if !found {
		t.Fatalf("created host not found in list")
	}

	// Get
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/hosts/%s/%s", serverURL, testNamespace, hostName))
	if err != nil {
		t.Fatalf("GET /hosts/:ns/:name failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var gotHost repositoryhost.RepositoryHost
	json.NewDecoder(resp.Body).Decode(&gotHost)
	if gotHost.BaseURL != "s3:https://s3.example.com/bucket" {
		t.Fatalf("expected baseUrl 's3:https://s3.example.com/bucket', got %q", gotHost.BaseURL)
	}

	// Update
	updateReq := repositoryhost.UpdateHostRequest{
		BaseURL: "s3:https://s3.updated.com/bucket",
	}
	body, _ = json.Marshal(updateReq)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/v1/hosts/%s/%s", serverURL, testNamespace, hostName), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /hosts/:ns/:name failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
	var updatedHost repositoryhost.RepositoryHost
	json.NewDecoder(resp.Body).Decode(&updatedHost)
	if updatedHost.BaseURL != "s3:https://s3.updated.com/bucket" {
		t.Fatalf("expected updated baseUrl, got %q", updatedHost.BaseURL)
	}

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/hosts/%s/%s", serverURL, testNamespace, hostName), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /hosts/:ns/:name failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify deleted
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/hosts/%s/%s", serverURL, testNamespace, hostName))
	if err != nil {
		t.Fatalf("GET after delete failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
}

func TestE2E_RepositoryCRUD(t *testing.T) {
	hostName := "e2e-repo-host"
	repoName := "e2e-test-repo"

	// Create host first
	createHost := repositoryhost.CreateHostRequest{
		Name:      hostName,
		Namespace: testNamespace,
		BaseURL:   "s3:https://s3.example.com/bucket",
	}
	body, _ := json.Marshal(createHost)
	resp, err := http.Post(serverURL+"/api/v1/hosts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /hosts failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for host, got %d", resp.StatusCode)
	}

	// Create repository
	createRepo := repository.CreateRepositoryRequest{
		Name:           repoName,
		Namespace:      testNamespace,
		HostRef:        testNamespace + "/" + hostName,
		Path:           "backups/test",
		ResticPassword: "test-password",
	}
	body, _ = json.Marshal(createRepo)
	resp, err = http.Post(serverURL+"/api/v1/repositories", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /repositories failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
	var createdRepo repository.Repository
	json.NewDecoder(resp.Body).Decode(&createdRepo)
	if createdRepo.Name != repoName {
		t.Fatalf("expected name %q, got %q", repoName, createdRepo.Name)
	}
	expectedURL := "s3:https://s3.example.com/bucket/backups/test"
	if createdRepo.URL != expectedURL {
		t.Fatalf("expected URL %q, got %q", expectedURL, createdRepo.URL)
	}

	// Get
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/repositories/%s/%s", serverURL, testNamespace, repoName))
	if err != nil {
		t.Fatalf("GET /repositories/:ns/:name failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// List
	resp, err = http.Get(serverURL + "/api/v1/repositories")
	if err != nil {
		t.Fatalf("GET /repositories failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var repos []*repository.Repository
	json.NewDecoder(resp.Body).Decode(&repos)
	found := false
	for _, r := range repos {
		if r.Name == repoName && r.Namespace == testNamespace {
			found = true
		}
	}
	if !found {
		t.Fatalf("created repository not found in list")
	}

	// Update
	updateRepo := repository.UpdateRepositoryRequest{
		Path: "backups/updated",
	}
	body, _ = json.Marshal(updateRepo)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/v1/repositories/%s/%s", serverURL, testNamespace, repoName), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /repositories/:ns/:name failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
	var updatedRepo repository.Repository
	json.NewDecoder(resp.Body).Decode(&updatedRepo)
	expectedURL = "s3:https://s3.example.com/bucket/backups/updated"
	if updatedRepo.URL != expectedURL {
		t.Fatalf("expected URL %q after update, got %q", expectedURL, updatedRepo.URL)
	}

	// Delete repository
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/repositories/%s/%s", serverURL, testNamespace, repoName), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /repositories/:ns/:name failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Clean up host
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/hosts/%s/%s", serverURL, testNamespace, hostName), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE host cleanup failed: %v", err)
	}
	resp.Body.Close()
}

func TestE2E_BackupJobCRUD(t *testing.T) {
	hostName := "e2e-bj-host"
	repoName := "e2e-bj-repo"
	jobName := "e2e-test-job"

	// Create host
	createHost := repositoryhost.CreateHostRequest{
		Name:      hostName,
		Namespace: testNamespace,
		BaseURL:   "s3:https://s3.example.com/bucket",
	}
	body, _ := json.Marshal(createHost)
	resp, err := http.Post(serverURL+"/api/v1/hosts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /hosts failed: %v", err)
	}
	resp.Body.Close()

	// Create repository
	createRepo := repository.CreateRepositoryRequest{
		Name:           repoName,
		Namespace:      testNamespace,
		HostRef:        testNamespace + "/" + hostName,
		Path:           "backups/bj-test",
		ResticPassword: "test-password",
	}
	body, _ = json.Marshal(createRepo)
	resp, err = http.Post(serverURL+"/api/v1/repositories", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /repositories failed: %v", err)
	}
	resp.Body.Close()

	// Create backup job
	createJob := backupjob.CreateBackupJobRequest{
		Name:          jobName,
		Namespace:     testNamespace,
		Type:          backupjob.BackupJobTypePSQL,
		Schedule:      "0 2 * * *",
		RepositoryRef: testNamespace + "/" + repoName,
		PSQL: &backupjob.PSQLConfig{
			Hostname: "postgres.default.svc",
			Port:     5432,
			Username: "backup_user",
			Database: "mydb",
			Password: "secret123",
		},
	}
	body, _ = json.Marshal(createJob)
	resp, err = http.Post(serverURL+"/api/v1/backupjobs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /backupjobs failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
	var createdJob backupjob.BackupJob
	json.NewDecoder(resp.Body).Decode(&createdJob)
	if createdJob.Name != jobName {
		t.Fatalf("expected name %q, got %q", jobName, createdJob.Name)
	}
	if createdJob.Type != backupjob.BackupJobTypePSQL {
		t.Fatalf("expected type psql, got %q", createdJob.Type)
	}
	if createdJob.PSQL == nil {
		t.Fatal("expected PSQL config to be set")
	}
	// Password should be cleared after credential secret is created
	if createdJob.PSQL.CredentialSecretRef == "" {
		t.Fatal("expected credentialSecretRef to be set after creation")
	}

	// Verify credential secret was created in Kubernetes
	secretName := jobName + "-psql-credentials"
	_, err = clientSet.CoreV1().Secrets(testNamespace).Get(t.Context(), secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected credential secret %q to exist: %v", secretName, err)
	}

	// List
	resp, err = http.Get(serverURL + "/api/v1/backupjobs")
	if err != nil {
		t.Fatalf("GET /backupjobs failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var jobs []*backupjob.BackupJob
	json.NewDecoder(resp.Body).Decode(&jobs)
	found := false
	for _, j := range jobs {
		if j.Name == jobName && j.Namespace == testNamespace {
			found = true
		}
	}
	if !found {
		t.Fatalf("created backup job not found in list")
	}

	// Get
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/backupjobs/%s/%s", serverURL, testNamespace, jobName))
	if err != nil {
		t.Fatalf("GET /backupjobs/:ns/:name failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Update schedule
	updateJob := backupjob.UpdateBackupJobRequest{
		Schedule: "0 3 * * *",
	}
	body, _ = json.Marshal(updateJob)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/v1/backupjobs/%s/%s", serverURL, testNamespace, jobName), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /backupjobs/:ns/:name failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
	var updatedJob backupjob.BackupJob
	json.NewDecoder(resp.Body).Decode(&updatedJob)
	if updatedJob.Schedule != "0 3 * * *" {
		t.Fatalf("expected schedule '0 3 * * *', got %q", updatedJob.Schedule)
	}

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/backupjobs/%s/%s", serverURL, testNamespace, jobName), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /backupjobs/:ns/:name failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Clean up
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/repositories/%s/%s", serverURL, testNamespace, repoName), nil)
	http.DefaultClient.Do(req)
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/hosts/%s/%s", serverURL, testNamespace, hostName), nil)
	http.DefaultClient.Do(req)
}

func TestE2E_HostDeleteBlocked(t *testing.T) {
	hostName := "e2e-blocked-host"
	repoName := "e2e-blocked-repo"

	// Create host
	createHost := repositoryhost.CreateHostRequest{
		Name:      hostName,
		Namespace: testNamespace,
		BaseURL:   "s3:https://s3.example.com/bucket",
	}
	body, _ := json.Marshal(createHost)
	resp, err := http.Post(serverURL+"/api/v1/hosts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /hosts failed: %v", err)
	}
	resp.Body.Close()

	// Create repository referencing the host
	createRepo := repository.CreateRepositoryRequest{
		Name:           repoName,
		Namespace:      testNamespace,
		HostRef:        testNamespace + "/" + hostName,
		Path:           "backups/blocked",
		ResticPassword: "test-password",
	}
	body, _ = json.Marshal(createRepo)
	resp, err = http.Post(serverURL+"/api/v1/repositories", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /repositories failed: %v", err)
	}
	resp.Body.Close()

	// Try to delete host - should get 409 Conflict
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/hosts/%s/%s", serverURL, testNamespace, hostName), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /hosts/:ns/:name failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", resp.StatusCode)
	}
	var conflictBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&conflictBody)
	if conflictBody["error"] == nil {
		t.Fatal("expected error message in conflict response")
	}

	// Delete repo first
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/repositories/%s/%s", serverURL, testNamespace, repoName), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /repositories/:ns/:name failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Now delete host should succeed
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/hosts/%s/%s", serverURL, testNamespace, hostName), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /hosts/:ns/:name after repo deletion failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestE2E_SettingsCRUD(t *testing.T) {
	// Get current settings so we can restore them after the test
	resp, err := http.Get(serverURL + "/api/v1/settings")
	if err != nil {
		t.Fatalf("GET /settings failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
	var original settings.Settings
	json.NewDecoder(resp.Body).Decode(&original)

	// Restore original settings when done so other tests aren't affected
	t.Cleanup(func() {
		restoreTTL := original.JobTTLSeconds
		restoreClass := original.DefaultStorageClassName
		restoreReq := settings.UpdateSettingsRequest{
			DefaultStorageClassName: &restoreClass,
			JobTTLSeconds:           &restoreTTL,
		}
		body, _ := json.Marshal(restoreReq)
		req, _ := http.NewRequest(http.MethodPut, serverURL+"/api/v1/settings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		http.DefaultClient.Do(req)
	})

	if original.JobTTLSeconds != 604800 {
		t.Fatalf("expected default jobTTLSeconds 604800, got %d", original.JobTTLSeconds)
	}

	// Update settings
	newTTL := int32(3600)
	newClassName := "fast-storage"
	updateReq := settings.UpdateSettingsRequest{
		DefaultStorageClassName: &newClassName,
		JobTTLSeconds:           &newTTL,
	}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest(http.MethodPut, serverURL+"/api/v1/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /settings failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
	var updated settings.Settings
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated.DefaultStorageClassName != "fast-storage" {
		t.Fatalf("expected defaultStorageClassName 'fast-storage', got %q", updated.DefaultStorageClassName)
	}
	if updated.JobTTLSeconds != 3600 {
		t.Fatalf("expected jobTTLSeconds 3600, got %d", updated.JobTTLSeconds)
	}

	// Verify by re-reading
	resp, err = http.Get(serverURL + "/api/v1/settings")
	if err != nil {
		t.Fatalf("GET /settings after update failed: %v", err)
	}
	defer resp.Body.Close()
	var verified settings.Settings
	json.NewDecoder(resp.Body).Decode(&verified)
	if verified.DefaultStorageClassName != "fast-storage" {
		t.Fatalf("expected persisted defaultStorageClassName 'fast-storage', got %q", verified.DefaultStorageClassName)
	}
	if verified.JobTTLSeconds != 3600 {
		t.Fatalf("expected persisted jobTTLSeconds 3600, got %d", verified.JobTTLSeconds)
	}
}
