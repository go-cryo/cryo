package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-cryo/cryo/internal/repositoryhost"
)

func TestListHosts_Empty(t *testing.T) {
	ts := setupTestServer(t)
	w := ts.do("GET", "/api/v1/hosts")

	assertStatus(t, w, http.StatusOK)

	var hosts []*repositoryhost.RepositoryHost
	decodeJSON(t, w, &hosts)
	if len(hosts) != 0 {
		t.Fatalf("expected 0 hosts, got %d", len(hosts))
	}
}

func TestListHosts_Populated(t *testing.T) {
	ts := setupTestServer(t)
	seedHost(ts, "default", "minio", "s3:https://minio.example.com")
	seedHost(ts, "backup", "nas", "/mnt/backup")

	w := ts.do("GET", "/api/v1/hosts")
	assertStatus(t, w, http.StatusOK)

	var hosts []*repositoryhost.RepositoryHost
	decodeJSON(t, w, &hosts)
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestGetHost_Found(t *testing.T) {
	ts := setupTestServer(t)
	seedHost(ts, "default", "minio", "s3:https://minio.example.com")

	w := ts.do("GET", "/api/v1/hosts/default/minio")
	assertStatus(t, w, http.StatusOK)

	var host repositoryhost.RepositoryHost
	decodeJSON(t, w, &host)
	if host.Name != "minio" {
		t.Fatalf("expected host name 'minio', got %q", host.Name)
	}
	if host.Namespace != "default" {
		t.Fatalf("expected namespace 'default', got %q", host.Namespace)
	}
	if host.BaseURL != "s3:https://minio.example.com" {
		t.Fatalf("expected baseUrl 's3:https://minio.example.com', got %q", host.BaseURL)
	}
}

func TestGetHost_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	w := ts.do("GET", "/api/v1/hosts/default/nonexistent")
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreateHost_Success(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"name":    "minio",
		"baseUrl": "s3:https://minio.example.com",
	}
	w := ts.doJSON("POST", "/api/v1/hosts", body)
	assertStatus(t, w, http.StatusCreated)

	var host repositoryhost.RepositoryHost
	decodeJSON(t, w, &host)
	if host.Name != "minio" {
		t.Fatalf("expected name 'minio', got %q", host.Name)
	}
	if host.Type != repositoryhost.HostTypeS3 {
		t.Fatalf("expected type 's3', got %q", host.Type)
	}

	// Verify it was stored
	if _, ok := ts.hostProvider.hosts["default/minio"]; !ok {
		t.Fatal("host was not stored in provider")
	}
}

func TestCreateHost_WithNamespace(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"name":      "nas",
		"namespace": "backup",
		"baseUrl":   "/mnt/backup",
	}
	w := ts.doJSON("POST", "/api/v1/hosts", body)
	assertStatus(t, w, http.StatusCreated)

	var host repositoryhost.RepositoryHost
	decodeJSON(t, w, &host)
	if host.Namespace != "backup" {
		t.Fatalf("expected namespace 'backup', got %q", host.Namespace)
	}
}

func TestCreateHost_MissingName(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"baseUrl": "s3:https://minio.example.com",
	}
	w := ts.doJSON("POST", "/api/v1/hosts", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateHost_MissingBaseURL(t *testing.T) {
	ts := setupTestServer(t)

	body := map[string]string{
		"name": "minio",
	}
	w := ts.doJSON("POST", "/api/v1/hosts", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateHost_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)

	req := httptest.NewRequest("POST", "/api/v1/hosts", bytes.NewBufferString(`{invalid json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateHost_Success(t *testing.T) {
	ts := setupTestServer(t)
	seedHost(ts, "default", "minio", "s3:https://old.example.com")

	body := map[string]string{
		"baseUrl": "s3:https://new.example.com",
	}
	w := ts.doJSON("PUT", "/api/v1/hosts/default/minio", body)
	assertStatus(t, w, http.StatusOK)

	var host repositoryhost.RepositoryHost
	decodeJSON(t, w, &host)
	if host.BaseURL != "s3:https://new.example.com" {
		t.Fatalf("expected updated baseUrl, got %q", host.BaseURL)
	}
}

func TestUpdateHost_MissingFields(t *testing.T) {
	ts := setupTestServer(t)
	seedHost(ts, "default", "minio", "s3:https://minio.example.com")

	body := map[string]string{}
	w := ts.doJSON("PUT", "/api/v1/hosts/default/minio", body)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateHost_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	seedHost(ts, "default", "minio", "s3:https://minio.example.com")

	req := httptest.NewRequest("PUT", "/api/v1/hosts/default/minio", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteHost_Success(t *testing.T) {
	ts := setupTestServer(t)
	seedHost(ts, "default", "minio", "s3:https://minio.example.com")

	w := ts.do("DELETE", "/api/v1/hosts/default/minio")
	assertStatus(t, w, http.StatusNoContent)

	if _, ok := ts.hostProvider.hosts["default/minio"]; ok {
		t.Fatal("host was not deleted from provider")
	}
}

func TestDeleteHost_ReferencedByRepo(t *testing.T) {
	ts := setupTestServer(t)
	seedHost(ts, "default", "minio", "s3:https://minio.example.com")
	seedRepo(ts, "default", "my-repo", "default/minio", "/backups")

	w := ts.do("DELETE", "/api/v1/hosts/default/minio")
	assertStatus(t, w, http.StatusConflict)

	var resp map[string]interface{}
	decodeJSON(t, w, &resp)
	if _, ok := resp["repositories"]; !ok {
		t.Fatal("expected 'repositories' key in conflict response")
	}

	// Verify host was NOT deleted
	if _, ok := ts.hostProvider.hosts["default/minio"]; !ok {
		t.Fatal("host should not have been deleted when referenced by a repository")
	}
}
