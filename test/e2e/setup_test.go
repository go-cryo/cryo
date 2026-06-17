//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-cryo/cryo/internal/auth"
	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/go-cryo/cryo/internal/executor"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/go-cryo/cryo/internal/repositoryhost"
	"github.com/go-cryo/cryo/internal/scheduler"
	"github.com/go-cryo/cryo/internal/server"
	"github.com/go-cryo/cryo/internal/settings"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	adminSecretName   = "cryo-admin-credentials"
	sessionSecretName = "cryo-session-keys"
	adminUsername     = "admin"
)

var (
	testNamespace  string
	clientSet      kubernetes.Interface
	dynamicClient  dynamic.Interface
	hostProvider   repositoryhost.Provider
	repoProvider   repository.Provider
	bjProvider     backupjob.Provider
	settProvider   settings.Provider
	runStore       *executor.RunStore
	sched          *scheduler.Scheduler
	serverURL      string
	testHTTPServer *httptest.Server
	// authClient carries the admin session cookie. The e2e suite runs with
	// BasicAuth enabled, so every protected API call goes through it.
	authClient *http.Client
)

func TestMain(m *testing.M) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build kubeconfig: %v\n", err)
		os.Exit(1)
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create clientset: %v\n", err)
		os.Exit(1)
	}
	clientSet = cs

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dynamic client: %v\n", err)
		os.Exit(1)
	}
	dynamicClient = dynClient

	testNamespace = fmt.Sprintf("cryo-e2e-%d", rand.Intn(10000))
	ctx := context.Background()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
	_, err = clientSet.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create namespace %s: %v\n", testNamespace, err)
		os.Exit(1)
	}

	// Deploy test infrastructure (RustFS, PostgreSQL, test PVC)
	if err := deployTestInfrastructure(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to deploy test infrastructure: %v\n", err)
		clientSet.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})
		os.Exit(1)
	}

	// Create providers
	hostProvider = repositoryhost.NewKubernetesProvider(cs, testNamespace)
	repoProvider = repository.NewKubernetesProvider(cs, testNamespace, hostProvider)
	bjProvider = backupjob.NewKubernetesProvider(cs, testNamespace)
	settProvider = settings.NewKubernetesProvider(cs, testNamespace)

	// Get backup images from env (set by CI or developer)
	psqlImage := envOrDefault("CRYO_PSQL_IMAGE", "localhost:5001/cryo-psql:test")
	s3Image := envOrDefault("CRYO_S3_IMAGE", "localhost:5001/cryo-s3:test")
	pvcImage := envOrDefault("CRYO_PVC_IMAGE", "localhost:5001/cryo-pvc:test")

	// Create executor components
	runStore = executor.NewRunStore(cs)
	pvcOrch := executor.NewPVCOrchestrator(cs, dynClient, repoProvider, pvcImage, settProvider)
	exec := executor.NewExecutor(cs, repoProvider, runStore, pvcOrch, &executor.ExecutorOptions{
		PSQLBackupImage: psqlImage,
		S3BackupImage:   s3Image,
		PVCBackupImage:  pvcImage,
	}, settProvider)

	// Create scheduler
	sched = scheduler.NewScheduler(bjProvider, exec)
	sched.Start()

	// Create server with all components wired
	srv, err := server.NewServer(&server.ServerOptions{
		ServiceVersion:     "test",
		DevMode:            false,
		Port:               0,
		ApiBaseUrl:         "/api/v1",
		HealthEndpoint:     "/health",
		StaticHosting:      false,
		HostProvider:       hostProvider,
		RepositoryProvider: repoProvider,
		BackupJobProvider:  bjProvider,
		Scheduler:          sched,
		RunStore:           runStore,
		SettingsProvider:   settProvider,
	})
	if err != nil {
		fatal(ctx, "Failed to create server: %v\n", err)
	}

	// Enable BasicAuth so the suite exercises the auth-protected API surface
	// exactly as production does. DevMode=true on the handler keeps the session
	// cookie non-Secure so it survives the plain-HTTP httptest server.
	sessionKeys, err := auth.EnsureSessionKeys(cs, testNamespace, sessionSecretName)
	if err != nil {
		fatal(ctx, "Failed to ensure session keys: %v\n", err)
	}
	basicAuthHandler, err := auth.NewBasicAuthHandler(srv.Engine, cs, testNamespace, &auth.BasicAuthHandlerOptions{
		AdminUsername:   adminUsername,
		AdminSecretName: adminSecretName,
		SessionKeys:     sessionKeys,
		DevMode:         true,
		ApiBaseUrl:      "/api/v1",
	})
	if err != nil {
		fatal(ctx, "Failed to set up basic auth: %v\n", err)
	}
	srv.BasicAuthHandler = basicAuthHandler

	srv.RegisterRoutes()

	testHTTPServer = httptest.NewServer(srv.Engine)
	serverURL = testHTTPServer.URL

	// Log in as the bootstrapped admin and reuse the session for all tests.
	authClient, err = loginAdmin(ctx)
	if err != nil {
		fatal(ctx, "Failed to log in admin: %v\n", err)
	}

	code := m.Run()

	testHTTPServer.Close()
	sched.Stop()
	clientSet.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})

	os.Exit(code)
}

// fatal tears down the test namespace and exits non-zero.
func fatal(ctx context.Context, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	if sched != nil {
		sched.Stop()
	}
	clientSet.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})
	os.Exit(1)
}

// deployTestInfrastructure deploys RustFS, PostgreSQL, and a test PVC into the
// test namespace, waits for them to be ready, and seeds them with test data.
func deployTestInfrastructure(ctx context.Context) error {
	mDir := filepath.Join("manifests")

	// Apply manifests
	if err := applyManifest(ctx, filepath.Join(mDir, "rustfs.yaml")); err != nil {
		return fmt.Errorf("deploying rustfs: %w", err)
	}
	if err := applyManifest(ctx, filepath.Join(mDir, "postgres.yaml")); err != nil {
		return fmt.Errorf("deploying postgres: %w", err)
	}
	if err := applyManifest(ctx, filepath.Join(mDir, "test-pvc.yaml")); err != nil {
		return fmt.Errorf("deploying test pvc: %w", err)
	}

	// Wait for deployments to be ready
	fmt.Println("Waiting for RustFS deployment...")
	if err := waitForDeploymentReady(ctx, "rustfs", 3*time.Minute); err != nil {
		return fmt.Errorf("waiting for rustfs: %w", err)
	}
	fmt.Println("Waiting for PostgreSQL deployment...")
	if err := waitForDeploymentReady(ctx, "postgres", 3*time.Minute); err != nil {
		return fmt.Errorf("waiting for postgres: %w", err)
	}

	// Seed RustFS: create buckets and upload test data
	fmt.Println("Seeding RustFS buckets...")
	if err := seedRustFS(ctx); err != nil {
		return fmt.Errorf("seeding rustfs: %w", err)
	}

	// Seed PostgreSQL: create test table and data
	fmt.Println("Seeding PostgreSQL data...")
	if err := seedPostgres(ctx); err != nil {
		return fmt.Errorf("seeding postgres: %w", err)
	}

	// Seed test PVC with files
	fmt.Println("Seeding test PVC...")
	if err := seedTestPVC(ctx); err != nil {
		return fmt.Errorf("seeding test pvc: %w", err)
	}

	fmt.Println("Test infrastructure ready")
	return nil
}
