package repositoryhost

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesProvider_CRUD(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	// Create
	host, err := provider.Create(ctx, &CreateHostRequest{
		Name:               "my-s3",
		Namespace:          "test-ns",
		BaseURL:            "s3:https://bucket.s3.amazonaws.com",
		AwsAccessKeyID:     "AKID",
		AwsSecretAccessKey: "secret",
		AwsDefaultRegion:   "us-east-1",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if host.Name != "my-s3" {
		t.Errorf("expected name my-s3, got %s", host.Name)
	}
	if host.Namespace != "test-ns" {
		t.Errorf("expected namespace test-ns, got %s", host.Namespace)
	}
	if host.Type != HostTypeS3 {
		t.Errorf("expected type s3, got %s", host.Type)
	}
	if host.BaseURL != "s3:https://bucket.s3.amazonaws.com" {
		t.Errorf("expected baseURL s3:https://bucket.s3.amazonaws.com, got %s", host.BaseURL)
	}

	// List
	hosts, err := provider.List(ctx)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}

	// Get
	host, err = provider.Get(ctx, "test-ns", "my-s3")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if host.Credentials["AWS_ACCESS_KEY_ID"] != "AKID" {
		t.Errorf("expected AWS_ACCESS_KEY_ID=AKID, got %s", host.Credentials["AWS_ACCESS_KEY_ID"])
	}
	if host.Credentials["AWS_SECRET_ACCESS_KEY"] != "secret" {
		t.Errorf("expected AWS_SECRET_ACCESS_KEY=secret, got %s", host.Credentials["AWS_SECRET_ACCESS_KEY"])
	}
	if host.Credentials["AWS_DEFAULT_REGION"] != "us-east-1" {
		t.Errorf("expected AWS_DEFAULT_REGION=us-east-1, got %s", host.Credentials["AWS_DEFAULT_REGION"])
	}

	// Update
	host, err = provider.Update(ctx, "test-ns", "my-s3", &UpdateHostRequest{
		BaseURL: "s3:https://new-bucket.s3.amazonaws.com",
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if host.BaseURL != "s3:https://new-bucket.s3.amazonaws.com" {
		t.Errorf("expected updated baseURL, got %s", host.BaseURL)
	}
	// Credentials should still be present
	if host.Credentials["AWS_ACCESS_KEY_ID"] != "AKID" {
		t.Errorf("expected AWS_ACCESS_KEY_ID preserved, got %s", host.Credentials["AWS_ACCESS_KEY_ID"])
	}

	// Delete
	err = provider.Delete(ctx, "test-ns", "my-s3")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	hosts, err = provider.List(ctx)
	if err != nil {
		t.Fatalf("List after delete error: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts after delete, got %d", len(hosts))
	}
}

func TestKubernetesProvider_NamespaceFallback(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	// Provider with namespace "provider-ns"
	provider := NewKubernetesProvider(clientset, "provider-ns")
	ctx := context.Background()

	// Create with empty namespace — should fall back to provider namespace
	host, err := provider.Create(ctx, &CreateHostRequest{
		Name:    "fallback-host",
		BaseURL: "rest:https://backup.example.com",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if host.Namespace != "provider-ns" {
		t.Errorf("expected namespace provider-ns, got %s", host.Namespace)
	}

	// Provider with empty namespace — should fall back to "default"
	provider2 := NewKubernetesProvider(clientset, "")
	host2, err := provider2.Create(ctx, &CreateHostRequest{
		Name:    "default-host",
		BaseURL: "/local/path",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if host2.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", host2.Namespace)
	}
}

func TestKubernetesProvider_SecretLabels(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	_, err := provider.Create(ctx, &CreateHostRequest{
		Name:    "label-test",
		BaseURL: "s3:https://bucket.s3.amazonaws.com",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Fetch the raw secret to check labels
	secret, err := clientset.CoreV1().Secrets("test-ns").Get(ctx, "label-test", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get secret error: %v", err)
	}

	labelVal, ok := secret.Labels["go-cryo.github.com/repository-host"]
	if !ok {
		t.Error("expected label go-cryo.github.com/repository-host to be present")
	}
	if labelVal != "true" {
		t.Errorf("expected label value true, got %s", labelVal)
	}
}

func TestKubernetesProvider_HostTypes(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	tests := []struct {
		name     string
		baseURL  string
		wantType HostType
	}{
		{"s3-host", "s3:https://bucket.s3.amazonaws.com", HostTypeS3},
		{"sftp-host", "sftp:user@host:/path", HostTypeSFTP},
		{"rest-host", "rest:https://backup.example.com", HostTypeRest},
		{"local-host", "/local/backup/path", HostTypeLocal},
	}

	for _, tt := range tests {
		host, err := provider.Create(ctx, &CreateHostRequest{
			Name:    tt.name,
			BaseURL: tt.baseURL,
		})
		if err != nil {
			t.Fatalf("Create %s error: %v", tt.name, err)
		}
		if host.Type != tt.wantType {
			t.Errorf("%s: expected type %s, got %s", tt.name, tt.wantType, host.Type)
		}
	}
}
