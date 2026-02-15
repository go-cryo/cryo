package backupjob

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesProvider_CreatePSQL(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	job, err := provider.Create(ctx, &CreateBackupJobRequest{
		Name:          "psql-backup",
		Namespace:     "test-ns",
		Type:          BackupJobTypePSQL,
		Schedule:      "0 2 * * *",
		RepositoryRef: "test-ns/my-repo",
		PSQL: &PSQLConfig{
			Hostname: "postgres.svc",
			Port:     5432,
			Username: "admin",
			Database: "mydb",
			Password: "secret123",
		},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if job.Name != "psql-backup" {
		t.Errorf("expected name psql-backup, got %s", job.Name)
	}
	if job.Type != BackupJobTypePSQL {
		t.Errorf("expected type psql, got %s", job.Type)
	}
	if job.Schedule != "0 2 * * *" {
		t.Errorf("expected schedule 0 2 * * *, got %s", job.Schedule)
	}
	if job.PSQL == nil {
		t.Fatal("expected PSQL config to be set")
	}
	if job.PSQL.Hostname != "postgres.svc" {
		t.Errorf("expected hostname postgres.svc, got %s", job.PSQL.Hostname)
	}
	// Password should be cleared after secret creation
	if job.PSQL.Password != "" {
		t.Errorf("expected password to be cleared from config, got %s", job.PSQL.Password)
	}
	// CredentialSecretRef should be set
	expectedSecretRef := "psql-backup-psql-credentials"
	if job.PSQL.CredentialSecretRef != expectedSecretRef {
		t.Errorf("expected credentialSecretRef %s, got %s", expectedSecretRef, job.PSQL.CredentialSecretRef)
	}

	// Verify credential secret was created
	secret, err := clientset.CoreV1().Secrets("test-ns").Get(ctx, expectedSecretRef, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("credential secret not found: %v", err)
	}
	if string(secret.Data["password"]) != "secret123" {
		t.Errorf("expected password secret123, got %s", string(secret.Data["password"]))
	}
	if string(secret.Data["hostname"]) != "postgres.svc" {
		t.Errorf("expected hostname postgres.svc, got %s", string(secret.Data["hostname"]))
	}
	if string(secret.Data["username"]) != "admin" {
		t.Errorf("expected username admin, got %s", string(secret.Data["username"]))
	}

	// Verify ConfigMap was created
	cm, err := clientset.CoreV1().ConfigMaps("test-ns").Get(ctx, "psql-backup", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}
	if _, ok := cm.Data["config"]; !ok {
		t.Error("expected 'config' key in ConfigMap data")
	}
}

func TestKubernetesProvider_CreateS3(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	job, err := provider.Create(ctx, &CreateBackupJobRequest{
		Name:          "s3-backup",
		Namespace:     "test-ns",
		Type:          BackupJobTypeS3,
		Schedule:      "0 3 * * *",
		RepositoryRef: "test-ns/my-repo",
		S3: &S3Config{
			Endpoint:  "s3.amazonaws.com",
			Bucket:    "my-bucket",
			AccessKey: "AKID",
			SecretKey: "SECRET",
		},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if job.Type != BackupJobTypeS3 {
		t.Errorf("expected type s3, got %s", job.Type)
	}
	if job.S3 == nil {
		t.Fatal("expected S3 config to be set")
	}
	// Inline credentials should be cleared
	if job.S3.AccessKey != "" {
		t.Errorf("expected AccessKey to be cleared, got %s", job.S3.AccessKey)
	}
	if job.S3.SecretKey != "" {
		t.Errorf("expected SecretKey to be cleared, got %s", job.S3.SecretKey)
	}
	if job.S3.CredentialsSecretRef == nil {
		t.Fatal("expected CredentialsSecretRef to be set")
	}

	// Verify credential secret
	secretName := "s3-backup-s3-credentials"
	secret, err := clientset.CoreV1().Secrets("test-ns").Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("credential secret not found: %v", err)
	}
	if string(secret.Data["accessKey"]) != "AKID" {
		t.Errorf("expected accessKey AKID, got %s", string(secret.Data["accessKey"]))
	}
	if string(secret.Data["secretKey"]) != "SECRET" {
		t.Errorf("expected secretKey SECRET, got %s", string(secret.Data["secretKey"]))
	}
}

func TestKubernetesProvider_CreatePVC(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	job, err := provider.Create(ctx, &CreateBackupJobRequest{
		Name:          "pvc-backup",
		Namespace:     "test-ns",
		Type:          BackupJobTypePVC,
		Schedule:      "0 4 * * *",
		RepositoryRef: "test-ns/my-repo",
		PVC: &PVCConfig{
			ClaimName:               "my-pvc",
			VolumeSnapshotClassName: "csi-snapclass",
			SnapshotRetention:       3,
		},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if job.Type != BackupJobTypePVC {
		t.Errorf("expected type pvc, got %s", job.Type)
	}
	if job.PVC == nil {
		t.Fatal("expected PVC config to be set")
	}
	if job.PVC.ClaimName != "my-pvc" {
		t.Errorf("expected claimName my-pvc, got %s", job.PVC.ClaimName)
	}
	if job.PVC.VolumeSnapshotClassName != "csi-snapclass" {
		t.Errorf("expected volumeSnapshotClassName csi-snapclass, got %s", job.PVC.VolumeSnapshotClassName)
	}
	if job.PVC.SnapshotRetention != 3 {
		t.Errorf("expected snapshotRetention 3, got %d", job.PVC.SnapshotRetention)
	}
}

func TestKubernetesProvider_Update_PreservesPassword(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	// Create with password
	_, err := provider.Create(ctx, &CreateBackupJobRequest{
		Name:          "preserve-pw",
		Namespace:     "test-ns",
		Type:          BackupJobTypePSQL,
		Schedule:      "0 2 * * *",
		RepositoryRef: "test-ns/my-repo",
		PSQL: &PSQLConfig{
			Hostname: "postgres.svc",
			Port:     5432,
			Username: "admin",
			Database: "mydb",
			Password: "original-pass",
		},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Verify password is in secret
	secretName := "preserve-pw-psql-credentials"
	secret, err := clientset.CoreV1().Secrets("test-ns").Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if string(secret.Data["password"]) != "original-pass" {
		t.Fatalf("expected original-pass, got %s", string(secret.Data["password"]))
	}

	// Update without password — password should be preserved
	_, err = provider.Update(ctx, "test-ns", "preserve-pw", &UpdateBackupJobRequest{
		PSQL: &PSQLConfig{
			Hostname: "new-postgres.svc",
			Username: "admin",
			Database: "mydb",
		},
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	// Verify password is still in secret
	secret, err = clientset.CoreV1().Secrets("test-ns").Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get secret after update: %v", err)
	}
	if string(secret.Data["password"]) != "original-pass" {
		t.Errorf("expected password to be preserved as original-pass, got %s", string(secret.Data["password"]))
	}
	// Hostname should be updated
	if string(secret.Data["hostname"]) != "new-postgres.svc" {
		t.Errorf("expected hostname to be updated to new-postgres.svc, got %s", string(secret.Data["hostname"]))
	}
}

func TestKubernetesProvider_Delete(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	_, err := provider.Create(ctx, &CreateBackupJobRequest{
		Name:          "delete-me",
		Namespace:     "test-ns",
		Type:          BackupJobTypePVC,
		Schedule:      "0 4 * * *",
		RepositoryRef: "test-ns/my-repo",
		PVC: &PVCConfig{
			ClaimName: "pvc1",
		},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Verify ConfigMap exists
	_, err = clientset.CoreV1().ConfigMaps("test-ns").Get(ctx, "delete-me", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected ConfigMap to exist: %v", err)
	}

	err = provider.Delete(ctx, "test-ns", "delete-me")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// Verify ConfigMap is gone
	_, err = clientset.CoreV1().ConfigMaps("test-ns").Get(ctx, "delete-me", metav1.GetOptions{})
	if err == nil {
		t.Error("expected ConfigMap to be deleted")
	}
}

func TestKubernetesProvider_Labels(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	provider := NewKubernetesProvider(clientset, "test-ns")
	ctx := context.Background()

	_, err := provider.Create(ctx, &CreateBackupJobRequest{
		Name:          "label-check",
		Namespace:     "test-ns",
		Type:          BackupJobTypePVC,
		Schedule:      "0 4 * * *",
		RepositoryRef: "test-ns/my-repo",
		PVC: &PVCConfig{
			ClaimName: "pvc1",
		},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	cm, err := clientset.CoreV1().ConfigMaps("test-ns").Get(ctx, "label-check", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get ConfigMap error: %v", err)
	}

	labelVal, ok := cm.Labels["go-cryo.github.com/config"]
	if !ok {
		t.Error("expected label go-cryo.github.com/config to be present")
	}
	if labelVal != "true" {
		t.Errorf("expected label value true, got %s", labelVal)
	}
}
