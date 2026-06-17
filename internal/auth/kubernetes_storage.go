package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	basicauth "github.com/mxcd/go-basicauth"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KubernetesStorage implements basicauth.Storage backed by a single Kubernetes Secret.
// Every read operation fetches the Secret fresh to ensure password changes via kubectl
// take effect immediately.
type KubernetesStorage struct {
	clientSet  kubernetes.Interface
	namespace  string
	secretName string
}

func NewKubernetesStorage(clientSet kubernetes.Interface, namespace, secretName string) *KubernetesStorage {
	if namespace == "" {
		namespace = "default"
	}
	return &KubernetesStorage{
		clientSet:  clientSet,
		namespace:  namespace,
		secretName: secretName,
	}
}

func (s *KubernetesStorage) CreateUser(user *basicauth.User) error {
	ctx := context.Background()

	username := ""
	if user.Username != nil {
		username = *user.Username
	}
	email := ""
	if user.Email != nil {
		email = *user.Email
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.secretName,
			Namespace: s.namespace,
			Labels: map[string]string{
				"go-cryo.github.com/auth-user": "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"USER_ID":       []byte(user.ID.String()),
			"USERNAME":      []byte(username),
			"EMAIL":         []byte(email),
			"PASSWORD_HASH": []byte(user.PasswordHash),
			"CREATED_AT":    []byte(user.CreatedAt.Format(time.RFC3339)),
			"UPDATED_AT":    []byte(user.UpdatedAt.Format(time.RFC3339)),
		},
	}

	_, err := s.clientSet.CoreV1().Secrets(s.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating user secret: %w", err)
	}
	return nil
}

func (s *KubernetesStorage) GetUserByUsername(username string) (*basicauth.User, error) {
	user, err := s.readUser()
	if err != nil {
		return nil, err
	}
	if user.Username == nil || *user.Username != username {
		return nil, basicauth.ErrUserNotFound
	}
	return user, nil
}

func (s *KubernetesStorage) GetUserByEmail(email string) (*basicauth.User, error) {
	user, err := s.readUser()
	if err != nil {
		return nil, err
	}
	if user.Email == nil || *user.Email != email {
		return nil, basicauth.ErrUserNotFound
	}
	return user, nil
}

func (s *KubernetesStorage) GetUserByID(id uuid.UUID) (*basicauth.User, error) {
	user, err := s.readUser()
	if err != nil {
		return nil, err
	}
	if user.ID != id {
		return nil, basicauth.ErrUserNotFound
	}
	return user, nil
}

func (s *KubernetesStorage) UpdateUser(user *basicauth.User) error {
	ctx := context.Background()

	secret, err := s.clientSet.CoreV1().Secrets(s.namespace).Get(ctx, s.secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting user secret for update: %w", err)
	}

	username := ""
	if user.Username != nil {
		username = *user.Username
	}
	email := ""
	if user.Email != nil {
		email = *user.Email
	}

	secret.Data["USER_ID"] = []byte(user.ID.String())
	secret.Data["USERNAME"] = []byte(username)
	secret.Data["EMAIL"] = []byte(email)
	secret.Data["PASSWORD_HASH"] = []byte(user.PasswordHash)
	secret.Data["UPDATED_AT"] = []byte(user.UpdatedAt.Format(time.RFC3339))

	_, err = s.clientSet.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating user secret: %w", err)
	}
	return nil
}

func (s *KubernetesStorage) DeleteUser(_ uuid.UUID) error {
	ctx := context.Background()
	if err := s.clientSet.CoreV1().Secrets(s.namespace).Delete(ctx, s.secretName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deleting user secret: %w", err)
	}
	return nil
}

func (s *KubernetesStorage) readUser() (*basicauth.User, error) {
	ctx := context.Background()

	secret, err := s.clientSet.CoreV1().Secrets(s.namespace).Get(ctx, s.secretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, basicauth.ErrUserNotFound
		}
		return nil, fmt.Errorf("getting user secret: %w", err)
	}

	return secretToUser(secret)
}

func secretToUser(secret *corev1.Secret) (*basicauth.User, error) {
	idStr := string(secret.Data["USER_ID"])
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("parsing user ID: %w", err)
	}

	username := string(secret.Data["USERNAME"])
	email := string(secret.Data["EMAIL"])

	createdAt, _ := time.Parse(time.RFC3339, string(secret.Data["CREATED_AT"]))
	updatedAt, _ := time.Parse(time.RFC3339, string(secret.Data["UPDATED_AT"]))

	user := &basicauth.User{
		ID:           id,
		PasswordHash: string(secret.Data["PASSWORD_HASH"]),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
	if username != "" {
		user.Username = &username
	}
	if email != "" {
		user.Email = &email
	}

	return user, nil
}
