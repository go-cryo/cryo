package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/go-cryo/cryo/internal/repositoryhost"
	"github.com/go-cryo/cryo/internal/settings"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- Mock Host Provider ---

type mockHostProvider struct {
	hosts map[string]*repositoryhost.RepositoryHost
}

func newMockHostProvider() *mockHostProvider {
	return &mockHostProvider{hosts: make(map[string]*repositoryhost.RepositoryHost)}
}

func (m *mockHostProvider) List(_ context.Context) ([]*repositoryhost.RepositoryHost, error) {
	result := make([]*repositoryhost.RepositoryHost, 0, len(m.hosts))
	for _, h := range m.hosts {
		result = append(result, h)
	}
	return result, nil
}

func (m *mockHostProvider) Get(_ context.Context, namespace, name string) (*repositoryhost.RepositoryHost, error) {
	key := namespace + "/" + name
	h, ok := m.hosts[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return h, nil
}

func (m *mockHostProvider) Create(_ context.Context, req *repositoryhost.CreateHostRequest) (*repositoryhost.RepositoryHost, error) {
	ns := req.Namespace
	if ns == "" {
		ns = "default"
	}
	h := &repositoryhost.RepositoryHost{
		Name:      req.Name,
		Namespace: ns,
		BaseURL:   req.BaseURL,
		Type:      repositoryhost.InferHostType(req.BaseURL),
	}
	m.hosts[ns+"/"+req.Name] = h
	return h, nil
}

func (m *mockHostProvider) Update(_ context.Context, namespace, name string, req *repositoryhost.UpdateHostRequest) (*repositoryhost.RepositoryHost, error) {
	key := namespace + "/" + name
	h, ok := m.hosts[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	if req.BaseURL != "" {
		h.BaseURL = req.BaseURL
		h.Type = repositoryhost.InferHostType(req.BaseURL)
	}
	return h, nil
}

func (m *mockHostProvider) Delete(_ context.Context, namespace, name string) error {
	key := namespace + "/" + name
	if _, ok := m.hosts[key]; !ok {
		return fmt.Errorf("not found: %s", key)
	}
	delete(m.hosts, key)
	return nil
}

// --- Mock Repository Provider ---

type mockRepoProvider struct {
	repos map[string]*repository.Repository
}

func newMockRepoProvider() *mockRepoProvider {
	return &mockRepoProvider{repos: make(map[string]*repository.Repository)}
}

func (m *mockRepoProvider) List(_ context.Context) ([]*repository.Repository, error) {
	result := make([]*repository.Repository, 0, len(m.repos))
	for _, r := range m.repos {
		result = append(result, r)
	}
	return result, nil
}

func (m *mockRepoProvider) Get(_ context.Context, namespace, name string) (*repository.Repository, error) {
	key := namespace + "/" + name
	r, ok := m.repos[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return r, nil
}

func (m *mockRepoProvider) Create(_ context.Context, req *repository.CreateRepositoryRequest) (*repository.Repository, error) {
	ns := req.Namespace
	if ns == "" {
		ns = "default"
	}
	r := &repository.Repository{
		Name:      req.Name,
		Namespace: ns,
		HostRef:   req.HostRef,
		Path:      req.Path,
	}
	m.repos[ns+"/"+req.Name] = r
	return r, nil
}

func (m *mockRepoProvider) Update(_ context.Context, namespace, name string, req *repository.UpdateRepositoryRequest) (*repository.Repository, error) {
	key := namespace + "/" + name
	r, ok := m.repos[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	if req.HostRef != "" {
		r.HostRef = req.HostRef
	}
	if req.Path != "" {
		r.Path = req.Path
	}
	return r, nil
}

func (m *mockRepoProvider) Delete(_ context.Context, namespace, name string) error {
	key := namespace + "/" + name
	if _, ok := m.repos[key]; !ok {
		return fmt.Errorf("not found: %s", key)
	}
	delete(m.repos, key)
	return nil
}

// --- Mock BackupJob Provider ---

type mockBackupJobProvider struct {
	jobs map[string]*backupjob.BackupJob
}

func newMockBackupJobProvider() *mockBackupJobProvider {
	return &mockBackupJobProvider{jobs: make(map[string]*backupjob.BackupJob)}
}

func (m *mockBackupJobProvider) List(_ context.Context) ([]*backupjob.BackupJob, error) {
	result := make([]*backupjob.BackupJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		result = append(result, j)
	}
	return result, nil
}

func (m *mockBackupJobProvider) Get(_ context.Context, namespace, name string) (*backupjob.BackupJob, error) {
	key := namespace + "/" + name
	j, ok := m.jobs[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return j, nil
}

func (m *mockBackupJobProvider) Create(_ context.Context, req *backupjob.CreateBackupJobRequest) (*backupjob.BackupJob, error) {
	ns := req.Namespace
	if ns == "" {
		ns = "default"
	}
	j := &backupjob.BackupJob{
		Name:          req.Name,
		Namespace:     ns,
		Type:          req.Type,
		Schedule:      req.Schedule,
		Suspend:       req.Suspend,
		RepositoryRef: req.RepositoryRef,
		Image:         req.Image,
		Retention:     req.Retention,
		PSQL:          req.PSQL,
		S3:            req.S3,
		PVC:           req.PVC,
	}
	m.jobs[ns+"/"+req.Name] = j
	return j, nil
}

func (m *mockBackupJobProvider) Update(_ context.Context, namespace, name string, req *backupjob.UpdateBackupJobRequest) (*backupjob.BackupJob, error) {
	key := namespace + "/" + name
	j, ok := m.jobs[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	if req.Schedule != "" {
		j.Schedule = req.Schedule
	}
	if req.Suspend != nil {
		j.Suspend = *req.Suspend
	}
	if req.RepositoryRef != "" {
		j.RepositoryRef = req.RepositoryRef
	}
	return j, nil
}

func (m *mockBackupJobProvider) Delete(_ context.Context, namespace, name string) error {
	key := namespace + "/" + name
	if _, ok := m.jobs[key]; !ok {
		return fmt.Errorf("not found: %s", key)
	}
	delete(m.jobs, key)
	return nil
}

// --- Mock Settings Provider ---

type mockSettingsProvider struct {
	settings *settings.Settings
}

func newMockSettingsProvider() *mockSettingsProvider {
	return &mockSettingsProvider{settings: settings.DefaultSettings()}
}

func (m *mockSettingsProvider) Get(_ context.Context) (*settings.Settings, error) {
	return m.settings, nil
}

func (m *mockSettingsProvider) Update(_ context.Context, req *settings.UpdateSettingsRequest) (*settings.Settings, error) {
	if req.DefaultStorageClassName != nil {
		m.settings.DefaultStorageClassName = *req.DefaultStorageClassName
	}
	if req.DefaultRetention != nil {
		m.settings.DefaultRetention = req.DefaultRetention
	}
	if req.JobTTLSeconds != nil {
		m.settings.JobTTLSeconds = *req.JobTTLSeconds
	}
	return m.settings, nil
}

// --- Test Server Helper ---

type testServer struct {
	router           *gin.Engine
	hostProvider     *mockHostProvider
	repoProvider     *mockRepoProvider
	backupProvider   *mockBackupJobProvider
	settingsProvider *mockSettingsProvider
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	hp := newMockHostProvider()
	rp := newMockRepoProvider()
	bp := newMockBackupJobProvider()
	sp := newMockSettingsProvider()

	srv, err := NewServer(&ServerOptions{
		ServiceVersion: "test",
		DevMode:        false,
		Port:           0,
		ApiBaseUrl:     "/api/v1",
		StaticHosting:  false,
		HealthEndpoint: "/health",
		HostProvider:   hp,
		RepositoryProvider: rp,
		BackupJobProvider:  bp,
		SettingsProvider:   sp,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Register only the API routes we need for testing, avoiding
	// event.RegisterWebsocketManager and web.RegisterUI which have
	// side effects (goroutines, embedded FS) unsuitable for unit tests.
	srv.registerHealthRoute()
	srv.registerVersionRoute()
	srv.registerHostRoutes()
	srv.registerRepositoryRoutes()
	srv.registerBackupJobRoutes()
	srv.registerSettingsRoutes()

	return &testServer{
		router:           srv.Engine,
		hostProvider:     hp,
		repoProvider:     rp,
		backupProvider:   bp,
		settingsProvider: sp,
	}
}

func (ts *testServer) request(method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Fatalf("expected status %d, got %d; body: %s", expected, w.Code, w.Body.String())
	}
}

func seedHost(ts *testServer, namespace, name, baseURL string) {
	ts.hostProvider.hosts[namespace+"/"+name] = &repositoryhost.RepositoryHost{
		Name:      name,
		Namespace: namespace,
		BaseURL:   baseURL,
		Type:      repositoryhost.InferHostType(baseURL),
	}
}

func seedRepo(ts *testServer, namespace, name, hostRef, path string) {
	ts.repoProvider.repos[namespace+"/"+name] = &repository.Repository{
		Name:      name,
		Namespace: namespace,
		HostRef:   hostRef,
		Path:      path,
	}
}

func seedBackupJob(ts *testServer, namespace, name string, jobType backupjob.BackupJobType, schedule, repoRef string) {
	ts.backupProvider.jobs[namespace+"/"+name] = &backupjob.BackupJob{
		Name:          name,
		Namespace:     namespace,
		Type:          jobType,
		Schedule:      schedule,
		RepositoryRef: repoRef,
	}
}

// setupTestServerNoScheduler creates a server with Scheduler and RunStore set to nil.
func setupTestServerNoScheduler(t *testing.T) *testServer {
	t.Helper()

	hp := newMockHostProvider()
	rp := newMockRepoProvider()
	bp := newMockBackupJobProvider()
	sp := newMockSettingsProvider()

	srv, err := NewServer(&ServerOptions{
		ServiceVersion:     "test",
		DevMode:            false,
		Port:               0,
		ApiBaseUrl:         "/api/v1",
		StaticHosting:      false,
		HealthEndpoint:     "/health",
		HostProvider:       hp,
		RepositoryProvider: rp,
		BackupJobProvider:  bp,
		SettingsProvider:   sp,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	srv.registerBackupJobRoutes()

	return &testServer{
		router:           srv.Engine,
		hostProvider:     hp,
		repoProvider:     rp,
		backupProvider:   bp,
		settingsProvider: sp,
	}
}

func (ts *testServer) do(method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	ts.router.ServeHTTP(w, req)
	return w
}

func (ts *testServer) doJSON(method, path string, body interface{}) *httptest.ResponseRecorder {
	return ts.request(method, path, body)
}

