package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func testSessionKeys(t *testing.T) *SessionKeys {
	t.Helper()
	sign := make([]byte, 64)
	enc := make([]byte, 32)
	if _, err := rand.Read(sign); err != nil {
		t.Fatalf("rand: %v", err)
	}
	if _, err := rand.Read(enc); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return &SessionKeys{SigningKey: sign, EncryptionKey: enc}
}

func newTestBasicAuth(t *testing.T) (*BasicAuthHandler, *gin.Engine, *fake.Clientset) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cs := fake.NewSimpleClientset()
	engine := gin.New()
	h, err := NewBasicAuthHandler(engine, cs, "cryo", &BasicAuthHandlerOptions{
		AdminUsername:   "admin",
		AdminSecretName: "cryo-admin-credentials",
		SessionKeys:     testSessionKeys(t),
		DevMode:         true, // keep the session cookie non-Secure for plain-HTTP httptest
		ApiBaseUrl:      "/api/v1",
	})
	if err != nil {
		t.Fatalf("NewBasicAuthHandler: %v", err)
	}
	return h, engine, cs
}

func adminPassword(t *testing.T, cs *fake.Clientset) string {
	t.Helper()
	secret, err := cs.CoreV1().Secrets("cryo").Get(t.Context(), "cryo-admin-credentials", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting admin secret: %v", err)
	}
	return string(secret.Data["PASSWORD"])
}

func TestBasicAuth_BootstrapsAdminSecret(t *testing.T) {
	_, _, cs := newTestBasicAuth(t)
	secret, err := cs.CoreV1().Secrets("cryo").Get(t.Context(), "cryo-admin-credentials", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("admin secret not created: %v", err)
	}
	if string(secret.Data["USERNAME"]) != "admin" {
		t.Fatalf("USERNAME = %q, want admin", secret.Data["USERNAME"])
	}
	if len(secret.Data["PASSWORD"]) == 0 {
		t.Fatal("plaintext PASSWORD not stored in admin secret")
	}
	if len(secret.Data["PASSWORD_HASH"]) == 0 {
		t.Fatal("PASSWORD_HASH not stored in admin secret")
	}
}

func TestBasicAuth_IdempotentBootstrap(t *testing.T) {
	// Re-running the handler against an already-bootstrapped cluster must not
	// rotate the admin password.
	gin.SetMode(gin.TestMode)
	cs := fake.NewSimpleClientset()
	keys := testSessionKeys(t)
	opts := &BasicAuthHandlerOptions{
		AdminUsername:   "admin",
		AdminSecretName: "cryo-admin-credentials",
		SessionKeys:     keys,
		DevMode:         true,
		ApiBaseUrl:      "/api/v1",
	}
	if _, err := NewBasicAuthHandler(gin.New(), cs, "cryo", opts); err != nil {
		t.Fatalf("first handler: %v", err)
	}
	pw1 := adminPassword(t, cs)
	if _, err := NewBasicAuthHandler(gin.New(), cs, "cryo", opts); err != nil {
		t.Fatalf("second handler: %v", err)
	}
	pw2 := adminPassword(t, cs)
	if pw1 != pw2 {
		t.Fatal("admin password was rotated on re-bootstrap")
	}
}

func TestBasicAuth_LoginAndCheckSession(t *testing.T) {
	h, engine, cs := newTestBasicAuth(t)
	password := adminPassword(t, cs)

	srv := httptest.NewServer(engine)
	defer srv.Close()

	// A request with no session cookie must not be authenticated.
	bare, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if h.CheckSession(bare) {
		t.Fatal("CheckSession returned true without a session cookie")
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	body, _ := json.Marshal(map[string]string{"identifier": "admin", "password": password})
	resp, err := client.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	u, _ := url.Parse(srv.URL)
	cookies := jar.Cookies(u)
	if len(cookies) == 0 {
		t.Fatal("no session cookie set after login")
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	if !h.CheckSession(req) {
		t.Fatal("CheckSession returned false for a valid session cookie")
	}
}

func TestBasicAuth_LoginWrongPassword(t *testing.T) {
	_, engine, _ := newTestBasicAuth(t)
	srv := httptest.NewServer(engine)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"identifier": "admin", "password": "definitely-wrong"})
	resp, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("login with wrong password returned 200")
	}
}

func TestBasicAuth_RegistrationBlocked(t *testing.T) {
	_, engine, _ := newTestBasicAuth(t)
	srv := httptest.NewServer(engine)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"username": "intruder", "password": "password123"})
	resp, err := http.Post(srv.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("register status = %d, want 403", resp.StatusCode)
	}
}
