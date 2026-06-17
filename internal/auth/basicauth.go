package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	basicauth "github.com/mxcd/go-basicauth"
	"github.com/rs/zerolog/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type BasicAuthHandlerOptions struct {
	AdminUsername    string
	AdminSecretName string
	SessionKeys     *SessionKeys
	DevMode         bool
	ApiBaseUrl      string // e.g. "/api/v1"
}

// BasicAuthHandler wraps go-basicauth and provides session checking for the
// combined auth middleware. Routes are registered during creation.
type BasicAuthHandler struct {
	handler      *basicauth.Handler
	sessionStore *sessions.CookieStore
	sessionName  string
}

// NewBasicAuthHandler creates the handler, registers routes on the engine, and
// bootstraps the admin user. Routes are registered at ApiBaseUrl+"/auth" (e.g.
// /api/v1/auth/login, /api/v1/auth/logout, /api/v1/auth/me).
func NewBasicAuthHandler(engine *gin.Engine, clientSet kubernetes.Interface, namespace string, opts *BasicAuthHandlerOptions) (*BasicAuthHandler, error) {
	storage := NewKubernetesStorage(clientSet, namespace, opts.AdminSecretName)

	if err := ensureAdminUser(clientSet, namespace, storage, opts.AdminUsername, opts.AdminSecretName); err != nil {
		return nil, fmt.Errorf("ensuring admin user: %w", err)
	}

	settings := basicauth.BasicAuthSettings{
		EnableUsernameLogin:  true,
		EnableEmailLogin:     false,
		SessionName:          "cryo_session",
		SessionExpiration:    24 * time.Hour,
		SessionSecretKey:     opts.SessionKeys.SigningKey,
		SessionEncryptionKey: opts.SessionKeys.EncryptionKey,
		CookieSecure:         !opts.DevMode,
		CookieHttpOnly:       true,
		CookieSameSite:       http.SameSiteLaxMode,
		CookiePath:           "/",
		// All paths are public at the basicauth middleware level.
		// Actual route protection is handled by the server's combinedAuth().
		PathRules: []basicauth.PathRule{
			{Type: basicauth.PublicPathPrefix, Path: "/", Access: basicauth.PathAccessPublic},
		},
		PasswordRequirements: basicauth.PasswordRequirements{
			MinLength: 8,
		},
	}

	// Block registration endpoint
	engine.Use(func(c *gin.Context) {
		if c.Request.Method == "POST" && c.Request.URL.Path == opts.ApiBaseUrl+"/auth/register" {
			c.JSON(http.StatusForbidden, gin.H{"error": "registration is disabled"})
			c.Abort()
			return
		}
		c.Next()
	})

	handler, err := basicauth.NewHandler(&basicauth.Options{
		Engine:               engine,
		AuthenticationBaseUrl: opts.ApiBaseUrl + "/auth",
		Storage:              storage,
		Settings:             &settings,
	})
	if err != nil {
		return nil, fmt.Errorf("creating basicauth handler: %w", err)
	}

	if err := handler.RegisterRoutes(); err != nil {
		return nil, fmt.Errorf("registering basicauth routes: %w", err)
	}

	// Create our own session store with the same keys for independent session checking.
	sessionStore := sessions.NewCookieStore(
		opts.SessionKeys.SigningKey,
		opts.SessionKeys.EncryptionKey,
	)
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   int(settings.SessionExpiration.Seconds()),
		Secure:   !opts.DevMode,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	log.Info().Msg("basic auth enabled")

	return &BasicAuthHandler{
		handler:      handler,
		sessionStore: sessionStore,
		sessionName:  "cryo_session",
	}, nil
}

// CheckSession returns true if the request has a valid BasicAuth session cookie.
func (h *BasicAuthHandler) CheckSession(r *http.Request) bool {
	session, err := h.sessionStore.Get(r, h.sessionName)
	if err != nil || session == nil {
		return false
	}
	userID, ok := session.Values["user_id"].(string)
	return ok && userID != ""
}

func ensureAdminUser(clientSet kubernetes.Interface, namespace string, storage *KubernetesStorage, username, secretName string) error {
	_, err := storage.GetUserByUsername(username)
	if err == nil {
		log.Info().Str("username", username).Msg("admin user already exists")
		return nil
	}

	if err != basicauth.ErrUserNotFound {
		return fmt.Errorf("checking admin user: %w", err)
	}

	password, err := generateRandomPassword(32)
	if err != nil {
		return fmt.Errorf("generating admin password: %w", err)
	}

	hash, err := basicauth.HashPassword(password, basicauth.DefaultPasswordHashingParams)
	if err != nil {
		return fmt.Errorf("hashing admin password: %w", err)
	}

	now := time.Now()
	user := &basicauth.User{
		ID:           uuid.New(),
		Username:     &username,
		PasswordHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := storage.CreateUser(user); err != nil {
		return fmt.Errorf("creating admin user: %w", err)
	}

	// Store plaintext password in the secret so operator can retrieve it
	ctx := context.Background()
	ns := namespace
	if ns == "" {
		ns = "default"
	}
	secret, err := clientSet.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("getting admin secret to add password: %w", err)
		}
	} else {
		secret.Data["PASSWORD"] = []byte(password)
		if _, err := clientSet.CoreV1().Secrets(ns).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("updating admin secret with password: %w", err)
		}
	}

	log.Info().
		Str("username", username).
		Str("secret", secretName).
		Msg("admin user created. Retrieve password with: kubectl get secret " + secretName + " -o jsonpath='{.data.PASSWORD}' | base64 -d")

	return nil
}

func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}
