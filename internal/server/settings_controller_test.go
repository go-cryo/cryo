package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-cryo/cryo/internal/settings"
)

func TestGetSettings(t *testing.T) {
	ts := setupTestServer(t)

	w := ts.do("GET", "/api/v1/settings")
	assertStatus(t, w, http.StatusOK)

	var s settings.Settings
	decodeJSON(t, w, &s)

	// Default settings from settings.DefaultSettings()
	if s.JobTTLSeconds != 604800 {
		t.Fatalf("expected default jobTTLSeconds=604800, got %d", s.JobTTLSeconds)
	}
}

func TestUpdateSettings(t *testing.T) {
	ts := setupTestServer(t)

	ttl := int32(3600)
	storageClass := "standard"
	body := map[string]interface{}{
		"jobTTLSeconds":           ttl,
		"defaultStorageClassName": storageClass,
		"defaultRetention": map[string]int{
			"keepLast":  5,
			"keepDaily": 7,
		},
	}
	w := ts.doJSON("PUT", "/api/v1/settings", body)
	assertStatus(t, w, http.StatusOK)

	var s settings.Settings
	decodeJSON(t, w, &s)
	if s.JobTTLSeconds != 3600 {
		t.Fatalf("expected jobTTLSeconds=3600, got %d", s.JobTTLSeconds)
	}
	if s.DefaultStorageClassName != "standard" {
		t.Fatalf("expected defaultStorageClassName='standard', got %q", s.DefaultStorageClassName)
	}
	if s.DefaultRetention == nil {
		t.Fatal("expected defaultRetention to be set")
	}
	if s.DefaultRetention.KeepLast != 5 {
		t.Fatalf("expected keepLast=5, got %d", s.DefaultRetention.KeepLast)
	}
	if s.DefaultRetention.KeepDaily != 7 {
		t.Fatalf("expected keepDaily=7, got %d", s.DefaultRetention.KeepDaily)
	}

	// Verify the update persisted by doing a GET
	w2 := ts.do("GET", "/api/v1/settings")
	assertStatus(t, w2, http.StatusOK)

	var s2 settings.Settings
	decodeJSON(t, w2, &s2)
	if s2.JobTTLSeconds != 3600 {
		t.Fatalf("expected persisted jobTTLSeconds=3600, got %d", s2.JobTTLSeconds)
	}
}

func TestUpdateSettings_InvalidBody(t *testing.T) {
	ts := setupTestServer(t)

	req := httptest.NewRequest("PUT", "/api/v1/settings", bytes.NewBufferString(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestUpdateSettings_PartialUpdate(t *testing.T) {
	ts := setupTestServer(t)

	// First update just TTL
	body := map[string]interface{}{
		"jobTTLSeconds": 1800,
	}
	w := ts.doJSON("PUT", "/api/v1/settings", body)
	assertStatus(t, w, http.StatusOK)

	var s settings.Settings
	decodeJSON(t, w, &s)
	if s.JobTTLSeconds != 1800 {
		t.Fatalf("expected jobTTLSeconds=1800, got %d", s.JobTTLSeconds)
	}
	// Other fields should retain defaults
	if s.DefaultStorageClassName != "" {
		t.Fatalf("expected empty defaultStorageClassName, got %q", s.DefaultStorageClassName)
	}
}
