//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestE2E_Auth_Info verifies the public auth-info endpoint reports BasicAuth on.
func TestE2E_Auth_Info(t *testing.T) {
	resp, err := http.Get(serverURL + "/api/v1/auth/info")
	if err != nil {
		t.Fatalf("GET /auth/info: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var info struct {
		BasicEnabled bool `json:"basicEnabled"`
		OidcEnabled  bool `json:"oidcEnabled"`
	}
	json.NewDecoder(resp.Body).Decode(&info)
	if !info.BasicEnabled {
		t.Fatal("expected basicEnabled=true")
	}
}

// TestE2E_Auth_ProtectedRequiresSession verifies a protected API route rejects
// unauthenticated requests and accepts them once a session is established.
func TestE2E_Auth_ProtectedRequiresSession(t *testing.T) {
	// Unauthenticated client (no cookie jar) must be rejected.
	bare := &http.Client{}
	resp, err := bare.Get(serverURL + "/api/v1/hosts")
	if err != nil {
		t.Fatalf("GET /hosts (unauth): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", resp.StatusCode)
	}

	// Fresh authenticated client (kept separate from the shared authClient so
	// the logout below doesn't disturb other tests).
	secret, err := clientSet.CoreV1().Secrets(testNamespace).Get(t.Context(), adminSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting admin secret: %v", err)
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{
		"identifier": string(secret.Data["USERNAME"]),
		"password":   string(secret.Data["PASSWORD"]),
	})
	resp, err = client.Post(serverURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", resp.StatusCode)
	}

	// Authenticated request succeeds.
	resp, err = client.Get(serverURL + "/api/v1/hosts")
	if err != nil {
		t.Fatalf("GET /hosts (auth): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with session, got %d", resp.StatusCode)
	}

	// After logout the same client is rejected again.
	resp, err = client.Post(serverURL+"/api/v1/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	resp.Body.Close()

	resp, err = client.Get(serverURL + "/api/v1/hosts")
	if err != nil {
		t.Fatalf("GET /hosts (post-logout): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}
