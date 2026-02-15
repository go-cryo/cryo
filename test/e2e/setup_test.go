//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	// Deploy test infrastructure (MinIO, PostgreSQL, test PVC)
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

	// No custom storage class needed: PVCs bind via pod scheduling
	// (WaitForFirstConsumer) and the executor no longer waits for PVC
	// binding before creating the job.

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
		SettingsProvider:    settProvider,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		sched.Stop()
		clientSet.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})
		os.Exit(1)
	}
	srv.RegisterRoutes()

	testHTTPServer = httptest.NewServer(srv.Engine)
	serverURL = testHTTPServer.URL

	code := m.Run()

	testHTTPServer.Close()
	sched.Stop()
	clientSet.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})

	os.Exit(code)
}

// deployTestInfrastructure deploys MinIO, PostgreSQL, and a test PVC into the
// test namespace, waits for them to be ready, and seeds them with test data.
func deployTestInfrastructure(ctx context.Context) error {
	mDir := filepath.Join("manifests")

	// Apply manifests
	if err := applyManifest(ctx, filepath.Join(mDir, "minio.yaml")); err != nil {
		return fmt.Errorf("deploying minio: %w", err)
	}
	if err := applyManifest(ctx, filepath.Join(mDir, "postgres.yaml")); err != nil {
		return fmt.Errorf("deploying postgres: %w", err)
	}
	if err := applyManifest(ctx, filepath.Join(mDir, "test-pvc.yaml")); err != nil {
		return fmt.Errorf("deploying test pvc: %w", err)
	}

	// Wait for deployments to be ready
	fmt.Println("Waiting for MinIO deployment...")
	if err := waitForDeploymentReady(ctx, "minio", 3*time.Minute); err != nil {
		return fmt.Errorf("waiting for minio: %w", err)
	}
	fmt.Println("Waiting for PostgreSQL deployment...")
	if err := waitForDeploymentReady(ctx, "postgres", 3*time.Minute); err != nil {
		return fmt.Errorf("waiting for postgres: %w", err)
	}

	// Seed MinIO: create buckets and upload test data
	fmt.Println("Seeding MinIO buckets...")
	if err := seedMinIO(ctx); err != nil {
		return fmt.Errorf("seeding minio: %w", err)
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

