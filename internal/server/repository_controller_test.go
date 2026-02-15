package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-cryo/cryo/internal/repository"
)

func TestListRepositories_Empty(t *testing.T) {
	ts := setupTestServer(t)
	w := ts.do("GET", "/api/v1/repositories")
	assertStatus(t, w, http.StatusOK)

	var repos []*repository.Repository
	decodeJSON(t, w, &repos)
	if len(repos) != 0 {
		t.Fatalf("expected 0 repositories, got %d", len(repos))
	}
}

func TestListRepositories_Populated(t *testing.T) {
	ts := setupTestServer(t)
	seedRepo(ts, "default", "repo1", "default/minio", "/backups/db")
	seedRepo(ts, "default", "repo2", "default/minio", "/backups/files")

	w := ts.do("GET", "/api/v1/repositories")
	assertStatus(t, w, http.StatusOK)

	var repos []*repository.Repository
	decodeJSON(t, w, &repos)
	if len(repos) != 2 {
		t.Fatalf("expected 2 repositories, got %d", len(repos))
	}
}

func TestGetRepository_Found(t *testing.T) {
	ts := setupTestServer(t)
	seedRepo(ts, "default", "my-repo", "default/minio", "/backups")

	w := ts.do("GET", "/api/v1/repositories/default/my-repo")
	assertStatus(t, w, http.StatusOK)

	var repo repository.Repository
	decodeJSON(t, w, &repo)
	if repo.Name != "my-repo" {
		t.Fatalf("expected name 'my-repo', got %q", repo.Name)
	}
	if repo.HostRef != "default/minio" {
		t.Fatalf("expected hostRef 'default/minio', got %q", repo.HostRef)
	}
	if repo.Path != "/backups" {
		t.Fatalf("expected path '/backups', got %q", repo.Path)
	}
}

func TestGetRepository_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	w := ts.do("GET", "/api/v1/repositories/default/nonexistent")
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreateRepository_Success(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"name":           "my-repo",
		"hostRef":        "default/minio",
		"path":           "/backups",
		"resticPassword": "secret123",
	}
	w := ts.doJSON("POST", "/api/v1/repositories", body)
	assertStatus(t, w, http.StatusCreated)

	var repo repository.Repository
	decodeJSON(t, w, &repo)
	if repo.Name != "my-repo" {
		t.Fatalf("expected name 'my-repo', got %q", repo.Name)
	}
	if repo.HostRef != "default/minio" {
		t.Fatalf("expected hostRef 'default/minio', got %q", repo.HostRef)
	}

	// Verify it was stored
	if _, ok := ts.repoProvider.repos["default/my-repo"]; !ok {
		t.Fatal("repository was not stored in provider")
	}
}

func TestCreateRepository_MissingName(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"hostRef":        "default/minio",
		"path":           "/backups",
		"resticPassword": "secret123",
	}
	w := ts.doJSON("POST", "/api/v1/repositories", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateRepository_MissingHostRef(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"name":           "my-repo",
		"path":           "/backups",
		"resticPassword": "secret123",
	}
	w := ts.doJSON("POST", "/api/v1/repositories", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateRepository_MissingPath(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"name":           "my-repo",
		"hostRef":        "default/minio",
		"resticPassword": "secret123",
	}
	w := ts.doJSON("POST", "/api/v1/repositories", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateRepository_MissingPassword(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"name":    "my-repo",
		"hostRef": "default/minio",
		"path":    "/backups",
	}
	w := ts.doJSON("POST", "/api/v1/repositories", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateRepository_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)

	req := httptest.NewRequest("POST", "/api/v1/repositories", bytes.NewBufferString(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateRepository_Success(t *testing.T) {
	ts := setupTestServer(t)
	seedRepo(ts, "default", "my-repo", "default/minio", "/old-path")

	body := map[string]string{
		"path": "/new-path",
	}
	w := ts.doJSON("PUT", "/api/v1/repositories/default/my-repo", body)
	assertStatus(t, w, http.StatusOK)

	var repo repository.Repository
	decodeJSON(t, w, &repo)
	if repo.Path != "/new-path" {
		t.Fatalf("expected updated path '/new-path', got %q", repo.Path)
	}
}

func TestUpdateRepository_MissingFields(t *testing.T) {
	ts := setupTestServer(t)
	seedRepo(ts, "default", "my-repo", "default/minio", "/backups")

	body := map[string]string{}
	w := ts.doJSON("PUT", "/api/v1/repositories/default/my-repo", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateRepository_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	seedRepo(ts, "default", "my-repo", "default/minio", "/backups")

	req := httptest.NewRequest("PUT", "/api/v1/repositories/default/my-repo", bytes.NewBufferString(`nope`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteRepository_Success(t *testing.T) {
	ts := setupTestServer(t)
	seedRepo(ts, "default", "my-repo", "default/minio", "/backups")

	w := ts.do("DELETE", "/api/v1/repositories/default/my-repo")
	assertStatus(t, w, http.StatusNoContent)

	if _, ok := ts.repoProvider.repos["default/my-repo"]; ok {
		t.Fatal("repository was not deleted from provider")
	}
}
