package repository

import (
	"context"
	"testing"

	"github.com/go-cryo/cryo/internal/repositoryhost"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func createHostSecret(t *testing.T, clientset *fake.Clientset, namespace, name, baseURL string, creds map[string][]byte) {
	t.Helper()
	data := map[string][]byte{
		"BASE_URL": []byte(baseURL),
	}
	for k, v := range creds {
		data[k] = v
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"go-cryo.github.com/repository-host": "true",
			},
		},
		Data: data,
	}
	_, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create host secret: %v", err)
	}
}

func TestKubernetesProvider_CRUD(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()

	// Create host secret first
	createHostSecret(t, clientset, "test-ns", "my-host", "s3:https://bucket.s3.amazonaws.com", map[string][]byte{
		"AWS_ACCESS_KEY_ID":     []byte("AKID"),
		"AWS_SECRET_ACCESS_KEY": []byte("secret-key"),
	})

	hostProvider := repositoryhost.NewKubernetesProvider(clientset, "test-ns")
	repoProvider := NewKubernetesProvider(clientset, "test-ns", hostProvider)

	// Create
	repo, err := repoProvider.Create(ctx, &CreateRepositoryRequest{
		Name:           "my-repo",
		Namespace:      "test-ns",
		HostRef:        "test-ns/my-host",
		Path:           "backups/db",
		ResticPassword: "restic-pass",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if repo.Name != "my-repo" {
		t.Errorf("expected name my-repo, got %s", repo.Name)
	}
	if repo.Namespace != "test-ns" {
		t.Errorf("expected namespace test-ns, got %s", repo.Namespace)
	}
	if repo.Type != RepositoryTypeS3 {
		t.Errorf("expected type s3, got %s", repo.Type)
	}
	if repo.URL != "s3:https://bucket.s3.amazonaws.com/backups/db" {
		t.Errorf("expected URL s3:https://bucket.s3.amazonaws.com/backups/db, got %s", repo.URL)
	}
	if repo.HostRef != "test-ns/my-host" {
		t.Errorf("expected hostRef test-ns/my-host, got %s", repo.HostRef)
	}
	if repo.Path != "backups/db" {
		t.Errorf("expected path backups/db, got %s", repo.Path)
	}

	// List
	repos, err := repoProvider.List(ctx)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	// Get
	repo, err = repoProvider.Get(ctx, "test-ns", "my-repo")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if repo.Credentials["RESTIC_PASSWORD"] != "restic-pass" {
		t.Errorf("expected RESTIC_PASSWORD=restic-pass, got %s", repo.Credentials["RESTIC_PASSWORD"])
	}
	if repo.Credentials["AWS_ACCESS_KEY_ID"] != "AKID" {
		t.Errorf("expected merged AWS_ACCESS_KEY_ID=AKID, got %s", repo.Credentials["AWS_ACCESS_KEY_ID"])
	}

	// Update path
	repo, err = repoProvider.Update(ctx, "test-ns", "my-repo", &UpdateRepositoryRequest{
		Path: "backups/new-db",
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if repo.URL != "s3:https://bucket.s3.amazonaws.com/backups/new-db" {
		t.Errorf("expected updated URL, got %s", repo.URL)
	}

	// Delete
	err = repoProvider.Delete(ctx, "test-ns", "my-repo")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	repos, err = repoProvider.List(ctx)
	if err != nil {
		t.Fatalf("List after delete error: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos after delete, got %d", len(repos))
	}
}

func TestKubernetesProvider_HostResolution(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()

	createHostSecret(t, clientset, "test-ns", "s3-host", "s3:https://my-bucket.s3.amazonaws.com", map[string][]byte{
		"AWS_ACCESS_KEY_ID":     []byte("key123"),
		"AWS_SECRET_ACCESS_KEY": []byte("secret456"),
		"AWS_DEFAULT_REGION":    []byte("eu-west-1"),
	})

	hostProvider := repositoryhost.NewKubernetesProvider(clientset, "test-ns")
	repoProvider := NewKubernetesProvider(clientset, "test-ns", hostProvider)

	repo, err := repoProvider.Create(ctx, &CreateRepositoryRequest{
		Name:           "resolved-repo",
		Namespace:      "test-ns",
		HostRef:        "test-ns/s3-host",
		Path:           "data/backups",
		ResticPassword: "pass",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// URL = host.BaseURL + "/" + path
	expectedURL := "s3:https://my-bucket.s3.amazonaws.com/data/backups"
	if repo.URL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, repo.URL)
	}

	// Credentials should be merged from host
	if repo.Credentials["RESTIC_REPOSITORY"] != expectedURL {
		t.Errorf("expected RESTIC_REPOSITORY credential, got %s", repo.Credentials["RESTIC_REPOSITORY"])
	}
	if repo.Credentials["RESTIC_PASSWORD"] != "pass" {
		t.Errorf("expected RESTIC_PASSWORD=pass, got %s", repo.Credentials["RESTIC_PASSWORD"])
	}
	if repo.Credentials["AWS_ACCESS_KEY_ID"] != "key123" {
		t.Errorf("expected AWS_ACCESS_KEY_ID=key123, got %s", repo.Credentials["AWS_ACCESS_KEY_ID"])
	}
	if repo.Credentials["AWS_SECRET_ACCESS_KEY"] != "secret456" {
		t.Errorf("expected AWS_SECRET_ACCESS_KEY=secret456, got %s", repo.Credentials["AWS_SECRET_ACCESS_KEY"])
	}
	if repo.Credentials["AWS_DEFAULT_REGION"] != "eu-west-1" {
		t.Errorf("expected AWS_DEFAULT_REGION=eu-west-1, got %s", repo.Credentials["AWS_DEFAULT_REGION"])
	}
}

func TestKubernetesProvider_LegacyFormat(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()

	// Create a legacy secret with RESTIC_REPOSITORY directly (no HOST_REF)
	legacySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-repo",
			Namespace: "test-ns",
			Labels: map[string]string{
				"go-cryo.github.com/repository": "true",
			},
		},
		Data: map[string][]byte{
			"RESTIC_REPOSITORY": []byte("s3:https://old-bucket.s3.amazonaws.com/backups"),
			"RESTIC_PASSWORD":   []byte("legacy-pass"),
			"AWS_ACCESS_KEY_ID": []byte("old-key"),
		},
	}
	_, err := clientset.CoreV1().Secrets("test-ns").Create(ctx, legacySecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create legacy secret: %v", err)
	}

	hostProvider := repositoryhost.NewKubernetesProvider(clientset, "test-ns")
	repoProvider := NewKubernetesProvider(clientset, "test-ns", hostProvider)

	repo, err := repoProvider.Get(ctx, "test-ns", "legacy-repo")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}

	if repo.URL != "s3:https://old-bucket.s3.amazonaws.com/backups" {
		t.Errorf("expected legacy URL, got %s", repo.URL)
	}
	if repo.Type != RepositoryTypeS3 {
		t.Errorf("expected type s3, got %s", repo.Type)
	}
	if repo.Credentials["RESTIC_PASSWORD"] != "legacy-pass" {
		t.Errorf("expected RESTIC_PASSWORD=legacy-pass, got %s", repo.Credentials["RESTIC_PASSWORD"])
	}
	if repo.Credentials["AWS_ACCESS_KEY_ID"] != "old-key" {
		t.Errorf("expected AWS_ACCESS_KEY_ID=old-key, got %s", repo.Credentials["AWS_ACCESS_KEY_ID"])
	}
	// Legacy format should have no host ref
	if repo.HostRef != "" {
		t.Errorf("expected empty HostRef for legacy format, got %s", repo.HostRef)
	}
}

func TestKubernetesProvider_SFTPUrl(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()

	// SFTP host with trailing colon
	createHostSecret(t, clientset, "test-ns", "sftp-host", "sftp:user@host:", nil)

	hostProvider := repositoryhost.NewKubernetesProvider(clientset, "test-ns")
	repoProvider := NewKubernetesProvider(clientset, "test-ns", hostProvider)

	repo, err := repoProvider.Create(ctx, &CreateRepositoryRequest{
		Name:           "sftp-repo",
		Namespace:      "test-ns",
		HostRef:        "test-ns/sftp-host",
		Path:           "backup/data",
		ResticPassword: "pass",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// SFTP with trailing colon: baseURL + path (no extra slash)
	expectedURL := "sftp:user@host:backup/data"
	if repo.URL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, repo.URL)
	}
	if repo.Type != RepositoryTypeSFTP {
		t.Errorf("expected type sftp, got %s", repo.Type)
	}

	// SFTP host without trailing colon
	createHostSecret(t, clientset, "test-ns", "sftp-host2", "sftp:user@host", nil)

	repo2, err := repoProvider.Create(ctx, &CreateRepositoryRequest{
		Name:           "sftp-repo2",
		Namespace:      "test-ns",
		HostRef:        "test-ns/sftp-host2",
		Path:           "backup/data",
		ResticPassword: "pass",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// SFTP without trailing colon: baseURL + "/" + path
	expectedURL2 := "sftp:user@host/backup/data"
	if repo2.URL != expectedURL2 {
		t.Errorf("expected URL %s, got %s", expectedURL2, repo2.URL)
	}
}

func TestKubernetesProvider_SecretLabels(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()

	createHostSecret(t, clientset, "test-ns", "label-host", "s3:https://bucket.s3.amazonaws.com", nil)

	hostProvider := repositoryhost.NewKubernetesProvider(clientset, "test-ns")
	repoProvider := NewKubernetesProvider(clientset, "test-ns", hostProvider)

	_, err := repoProvider.Create(ctx, &CreateRepositoryRequest{
		Name:           "label-repo",
		Namespace:      "test-ns",
		HostRef:        "test-ns/label-host",
		Path:           "data",
		ResticPassword: "pass",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	secret, err := clientset.CoreV1().Secrets("test-ns").Get(ctx, "label-repo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get secret error: %v", err)
	}

	labelVal, ok := secret.Labels["go-cryo.github.com/repository"]
	if !ok {
		t.Error("expected label go-cryo.github.com/repository to be present")
	}
	if labelVal != "true" {
		t.Errorf("expected label value true, got %s", labelVal)
	}
}
