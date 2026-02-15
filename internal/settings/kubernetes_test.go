package settings

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesProvider_GetCreatesDefaults(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	s, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}

	if s.JobTTLSeconds != 604800 {
		t.Errorf("expected default JobTTLSeconds 604800, got %d", s.JobTTLSeconds)
	}
	if s.DefaultStorageClassName != "" {
		t.Errorf("expected empty DefaultStorageClassName, got %s", s.DefaultStorageClassName)
	}
	if s.DefaultRetention != nil {
		t.Errorf("expected nil DefaultRetention, got %+v", s.DefaultRetention)
	}

	// Second Get should return the same defaults (reading from existing ConfigMap)
	s2, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("second Get error: %v", err)
	}
	if s2.JobTTLSeconds != 604800 {
		t.Errorf("expected JobTTLSeconds 604800 on second get, got %d", s2.JobTTLSeconds)
	}
}

func TestKubernetesProvider_Update(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	// Get first to create defaults
	_, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}

	// Update with new values
	newStorageClass := "fast-ssd"
	var newTTL int32 = 86400
	s, err := provider.Update(ctx, &UpdateSettingsRequest{
		DefaultStorageClassName: &newStorageClass,
		JobTTLSeconds:           &newTTL,
		DefaultRetention: &RetentionPolicy{
			KeepLast:  5,
			KeepDaily: 7,
		},
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	if s.DefaultStorageClassName != "fast-ssd" {
		t.Errorf("expected DefaultStorageClassName fast-ssd, got %s", s.DefaultStorageClassName)
	}
	if s.JobTTLSeconds != 86400 {
		t.Errorf("expected JobTTLSeconds 86400, got %d", s.JobTTLSeconds)
	}
	if s.DefaultRetention == nil {
		t.Fatal("expected DefaultRetention to be set")
	}
	if s.DefaultRetention.KeepLast != 5 {
		t.Errorf("expected KeepLast 5, got %d", s.DefaultRetention.KeepLast)
	}
	if s.DefaultRetention.KeepDaily != 7 {
		t.Errorf("expected KeepDaily 7, got %d", s.DefaultRetention.KeepDaily)
	}

	// Verify persisted by reading again
	s2, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("Get after update error: %v", err)
	}
	if s2.DefaultStorageClassName != "fast-ssd" {
		t.Errorf("expected persisted DefaultStorageClassName fast-ssd, got %s", s2.DefaultStorageClassName)
	}
	if s2.JobTTLSeconds != 86400 {
		t.Errorf("expected persisted JobTTLSeconds 86400, got %d", s2.JobTTLSeconds)
	}
}

func TestKubernetesProvider_JobTTLMinimum(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	// Get to create defaults
	_, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}

	// Update with JobTTLSeconds = 0
	var zeroTTL int32 = 0
	_, err = provider.Update(ctx, &UpdateSettingsRequest{
		JobTTLSeconds: &zeroTTL,
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	// On read, parseSettings should default 0 back to 604800
	s, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("Get after zero TTL update error: %v", err)
	}
	if s.JobTTLSeconds != 604800 {
		t.Errorf("expected JobTTLSeconds to be defaulted to 604800 when 0, got %d", s.JobTTLSeconds)
	}
}

func TestKubernetesProvider_EmptyNamespaceFallback(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	// Empty namespace should fall back to "default"
	provider := NewKubernetesProvider(clientset, "")
	ctx := context.Background()

	s, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if s.JobTTLSeconds != 604800 {
		t.Errorf("expected default JobTTLSeconds 604800, got %d", s.JobTTLSeconds)
	}
}

func TestKubernetesProvider_PartialUpdate(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	// Create defaults
	_, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}

	// Update only storage class
	sc := "premium"
	_, err = provider.Update(ctx, &UpdateSettingsRequest{
		DefaultStorageClassName: &sc,
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	s, err := provider.Get(ctx)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if s.DefaultStorageClassName != "premium" {
		t.Errorf("expected premium, got %s", s.DefaultStorageClassName)
	}
	// TTL should still be default
	if s.JobTTLSeconds != 604800 {
		t.Errorf("expected JobTTLSeconds to remain 604800, got %d", s.JobTTLSeconds)
	}
}
