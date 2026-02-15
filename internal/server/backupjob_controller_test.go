package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-cryo/cryo/internal/backupjob"
)

func TestListBackupJobs_Empty(t *testing.T) {
	ts := setupTestServer(t)
	w := ts.do("GET", "/api/v1/backupjobs")
	assertStatus(t, w, http.StatusOK)

	var jobs []*backupjob.BackupJob
	decodeJSON(t, w, &jobs)
	if len(jobs) != 0 {
		t.Fatalf("expected 0 backup jobs, got %d", len(jobs))
	}
}

func TestListBackupJobs_Populated(t *testing.T) {
	ts := setupTestServer(t)
	seedBackupJob(ts, "default", "db-backup", backupjob.BackupJobTypePSQL, "0 2 * * *", "default/repo1")
	seedBackupJob(ts, "default", "pvc-backup", backupjob.BackupJobTypePVC, "0 3 * * *", "default/repo2")

	w := ts.do("GET", "/api/v1/backupjobs")
	assertStatus(t, w, http.StatusOK)

	var jobs []*backupjob.BackupJob
	decodeJSON(t, w, &jobs)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 backup jobs, got %d", len(jobs))
	}
}

func TestGetBackupJob_Found(t *testing.T) {
	ts := setupTestServer(t)
	seedBackupJob(ts, "default", "db-backup", backupjob.BackupJobTypePSQL, "0 2 * * *", "default/repo1")

	w := ts.do("GET", "/api/v1/backupjobs/default/db-backup")
	assertStatus(t, w, http.StatusOK)

	var job backupjob.BackupJob
	decodeJSON(t, w, &job)
	if job.Name != "db-backup" {
		t.Fatalf("expected name 'db-backup', got %q", job.Name)
	}
	if job.Type != backupjob.BackupJobTypePSQL {
		t.Fatalf("expected type 'psql', got %q", job.Type)
	}
	if job.Schedule != "0 2 * * *" {
		t.Fatalf("expected schedule '0 2 * * *', got %q", job.Schedule)
	}
	if job.RepositoryRef != "default/repo1" {
		t.Fatalf("expected repositoryRef 'default/repo1', got %q", job.RepositoryRef)
	}
}

func TestGetBackupJob_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	w := ts.do("GET", "/api/v1/backupjobs/default/nonexistent")
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreateBackupJob_Success(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]interface{}{
		"name":          "db-backup",
		"type":          "psql",
		"schedule":      "0 2 * * *",
		"repositoryRef": "default/repo1",
		"psql": map[string]interface{}{
			"hostname": "postgres.default",
			"port":     5432,
			"username": "admin",
			"database": "mydb",
		},
	}
	w := ts.doJSON("POST", "/api/v1/backupjobs", body)
	assertStatus(t, w, http.StatusCreated)

	var job backupjob.BackupJob
	decodeJSON(t, w, &job)
	if job.Name != "db-backup" {
		t.Fatalf("expected name 'db-backup', got %q", job.Name)
	}
	if job.Type != backupjob.BackupJobTypePSQL {
		t.Fatalf("expected type 'psql', got %q", job.Type)
	}

	// Verify it was stored
	if _, ok := ts.backupProvider.jobs["default/db-backup"]; !ok {
		t.Fatal("backup job was not stored in provider")
	}
}

func TestCreateBackupJob_MissingName(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]interface{}{
		"type":          "psql",
		"schedule":      "0 2 * * *",
		"repositoryRef": "default/repo1",
	}
	w := ts.doJSON("POST", "/api/v1/backupjobs", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateBackupJob_MissingType(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]interface{}{
		"name":          "db-backup",
		"schedule":      "0 2 * * *",
		"repositoryRef": "default/repo1",
	}
	w := ts.doJSON("POST", "/api/v1/backupjobs", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateBackupJob_MissingSchedule(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]interface{}{
		"name":          "db-backup",
		"type":          "psql",
		"repositoryRef": "default/repo1",
	}
	w := ts.doJSON("POST", "/api/v1/backupjobs", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateBackupJob_MissingRepositoryRef(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]interface{}{
		"name":     "db-backup",
		"type":     "psql",
		"schedule": "0 2 * * *",
	}
	w := ts.doJSON("POST", "/api/v1/backupjobs", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateBackupJob_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)

	req := httptest.NewRequest("POST", "/api/v1/backupjobs", bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateBackupJob_Success(t *testing.T) {
	ts := setupTestServer(t)
	seedBackupJob(ts, "default", "db-backup", backupjob.BackupJobTypePSQL, "0 2 * * *", "default/repo1")

	body := map[string]interface{}{
		"schedule": "0 4 * * *",
	}
	w := ts.doJSON("PUT", "/api/v1/backupjobs/default/db-backup", body)
	assertStatus(t, w, http.StatusOK)

	var job backupjob.BackupJob
	decodeJSON(t, w, &job)
	if job.Schedule != "0 4 * * *" {
		t.Fatalf("expected updated schedule '0 4 * * *', got %q", job.Schedule)
	}
}

func TestUpdateBackupJob_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	seedBackupJob(ts, "default", "db-backup", backupjob.BackupJobTypePSQL, "0 2 * * *", "default/repo1")

	req := httptest.NewRequest("PUT", "/api/v1/backupjobs/default/db-backup", bytes.NewBufferString(`nope`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteBackupJob_Success(t *testing.T) {
	ts := setupTestServer(t)
	seedBackupJob(ts, "default", "db-backup", backupjob.BackupJobTypePSQL, "0 2 * * *", "default/repo1")

	w := ts.do("DELETE", "/api/v1/backupjobs/default/db-backup")
	assertStatus(t, w, http.StatusNoContent)

	if _, ok := ts.backupProvider.jobs["default/db-backup"]; ok {
		t.Fatal("backup job was not deleted from provider")
	}
}

func TestTriggerBackupJob_NoScheduler(t *testing.T) {
	ts := setupTestServerNoScheduler(t)
	seedBackupJob(ts, "default", "db-backup", backupjob.BackupJobTypePSQL, "0 2 * * *", "default/repo1")

	w := ts.doJSON("POST", "/api/v1/backupjobs/default/db-backup/trigger", nil)
	assertStatus(t, w, http.StatusServiceUnavailable)
}

func TestListBackupRuns_NoRunStore(t *testing.T) {
	ts := setupTestServerNoScheduler(t)
	seedBackupJob(ts, "default", "db-backup", backupjob.BackupJobTypePSQL, "0 2 * * *", "default/repo1")

	w := ts.do("GET", "/api/v1/backupjobs/default/db-backup/runs")
	assertStatus(t, w, http.StatusServiceUnavailable)
}
