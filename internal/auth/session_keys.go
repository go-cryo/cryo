package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rs/zerolog/log"
)

// SessionKeys holds the signing and encryption keys for session cookies.
type SessionKeys struct {
	SigningKey    []byte // 64 bytes for HMAC-SHA256
	EncryptionKey []byte // 32 bytes for AES-256
}

// EnsureSessionKeys retrieves or creates session keys in a Kubernetes Secret.
// On first boot, it generates random keys and stores them. On subsequent boots,
// it reads the existing keys from the Secret.
func EnsureSessionKeys(clientSet kubernetes.Interface, namespace, secretName string) (*SessionKeys, error) {
	if namespace == "" {
		namespace = "default"
	}

	ctx := context.Background()

	secret, err := clientSet.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		signingKey, err := base64.StdEncoding.DecodeString(string(secret.Data["SESSION_SIGNING_KEY"]))
		if err != nil {
			return nil, fmt.Errorf("decoding session signing key: %w", err)
		}
		encryptionKey, err := base64.StdEncoding.DecodeString(string(secret.Data["SESSION_ENCRYPTION_KEY"]))
		if err != nil {
			return nil, fmt.Errorf("decoding session encryption key: %w", err)
		}
		log.Info().Str("secret", secretName).Msg("loaded session keys from Kubernetes secret")
		return &SessionKeys{SigningKey: signingKey, EncryptionKey: encryptionKey}, nil
	}

	if !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("getting session keys secret: %w", err)
	}

	signingKey := make([]byte, 64)
	if _, err := rand.Read(signingKey); err != nil {
		return nil, fmt.Errorf("generating session signing key: %w", err)
	}

	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		return nil, fmt.Errorf("generating session encryption key: %w", err)
	}

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"go-cryo.github.com/auth-session-keys": "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"SESSION_SIGNING_KEY":    []byte(base64.StdEncoding.EncodeToString(signingKey)),
			"SESSION_ENCRYPTION_KEY": []byte(base64.StdEncoding.EncodeToString(encryptionKey)),
		},
	}

	if _, err := clientSet.CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("creating session keys secret: %w", err)
	}

	log.Info().Str("secret", secretName).Msg("generated and stored new session keys")
	return &SessionKeys{SigningKey: signingKey, EncryptionKey: encryptionKey}, nil
}
