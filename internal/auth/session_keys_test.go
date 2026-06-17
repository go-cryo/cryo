package auth

import (
	"bytes"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureSessionKeys_GenerateThenReload(t *testing.T) {
	cs := fake.NewSimpleClientset()

	keys, err := EnsureSessionKeys(cs, "cryo", "cryo-session-keys")
	if err != nil {
		t.Fatalf("first EnsureSessionKeys: %v", err)
	}
	if len(keys.SigningKey) != 64 {
		t.Fatalf("signing key length = %d, want 64", len(keys.SigningKey))
	}
	if len(keys.EncryptionKey) != 32 {
		t.Fatalf("encryption key length = %d, want 32", len(keys.EncryptionKey))
	}

	if _, err := cs.CoreV1().Secrets("cryo").Get(t.Context(), "cryo-session-keys", metav1.GetOptions{}); err != nil {
		t.Fatalf("session keys secret not persisted: %v", err)
	}

	// Second call must read the persisted keys, not regenerate them — otherwise
	// existing sessions would be invalidated on every restart.
	reloaded, err := EnsureSessionKeys(cs, "cryo", "cryo-session-keys")
	if err != nil {
		t.Fatalf("second EnsureSessionKeys: %v", err)
	}
	if !bytes.Equal(keys.SigningKey, reloaded.SigningKey) || !bytes.Equal(keys.EncryptionKey, reloaded.EncryptionKey) {
		t.Fatal("reloaded session keys differ from the originally generated keys")
	}
}

func TestEnsureSessionKeys_EmptyNamespaceDefaults(t *testing.T) {
	cs := fake.NewSimpleClientset()
	if _, err := EnsureSessionKeys(cs, "", "keys"); err != nil {
		t.Fatalf("EnsureSessionKeys: %v", err)
	}
	if _, err := cs.CoreV1().Secrets("default").Get(t.Context(), "keys", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected secret in 'default' namespace: %v", err)
	}
}
