package repositoryhost

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const labelSelector = "go-cryo.github.com/repository-host=true"

type KubernetesProvider struct {
	clientSet kubernetes.Interface
	namespace string
}

func NewKubernetesProvider(clientSet kubernetes.Interface, namespace string) *KubernetesProvider {
	return &KubernetesProvider{
		clientSet: clientSet,
		namespace: namespace,
	}
}

func (p *KubernetesProvider) List(ctx context.Context) ([]*RepositoryHost, error) {
	secrets, err := p.clientSet.CoreV1().Secrets(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing repository host secrets: %w", err)
	}

	hosts := make([]*RepositoryHost, 0, len(secrets.Items))
	for _, secret := range secrets.Items {
		host := secretToRepositoryHost(&secret)
		if host != nil {
			hosts = append(hosts, host)
		}
	}
	return hosts, nil
}

func (p *KubernetesProvider) Get(ctx context.Context, namespace, name string) (*RepositoryHost, error) {
	ns := namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}
	secret, err := p.clientSet.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting repository host secret %s/%s: %w", ns, name, err)
	}

	host := secretToRepositoryHost(secret)
	if host == nil {
		return nil, fmt.Errorf("secret %s/%s is missing BASE_URL", ns, name)
	}
	return host, nil
}

func (p *KubernetesProvider) Create(ctx context.Context, req *CreateHostRequest) (*RepositoryHost, error) {
	ns := req.Namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}

	data := map[string][]byte{
		"BASE_URL": []byte(req.BaseURL),
	}
	if req.AwsAccessKeyID != "" {
		data["AWS_ACCESS_KEY_ID"] = []byte(req.AwsAccessKeyID)
	}
	if req.AwsSecretAccessKey != "" {
		data["AWS_SECRET_ACCESS_KEY"] = []byte(req.AwsSecretAccessKey)
	}
	if req.AwsDefaultRegion != "" {
		data["AWS_DEFAULT_REGION"] = []byte(req.AwsDefaultRegion)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: ns,
			Labels: map[string]string{
				"go-cryo.github.com/repository-host": "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	created, err := p.clientSet.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating repository host secret %s/%s: %w", ns, req.Name, err)
	}

	return secretToRepositoryHost(created), nil
}

func (p *KubernetesProvider) Update(ctx context.Context, namespace, name string, req *UpdateHostRequest) (*RepositoryHost, error) {
	ns := namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}

	secret, err := p.clientSet.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting repository host secret %s/%s: %w", ns, name, err)
	}

	if req.BaseURL != "" {
		secret.Data["BASE_URL"] = []byte(req.BaseURL)
	}
	if req.AwsAccessKeyID != "" {
		secret.Data["AWS_ACCESS_KEY_ID"] = []byte(req.AwsAccessKeyID)
	}
	if req.AwsSecretAccessKey != "" {
		secret.Data["AWS_SECRET_ACCESS_KEY"] = []byte(req.AwsSecretAccessKey)
	}
	if req.AwsDefaultRegion != "" {
		secret.Data["AWS_DEFAULT_REGION"] = []byte(req.AwsDefaultRegion)
	}

	updated, err := p.clientSet.CoreV1().Secrets(ns).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("updating repository host secret %s/%s: %w", ns, name, err)
	}

	return secretToRepositoryHost(updated), nil
}

func (p *KubernetesProvider) Delete(ctx context.Context, namespace, name string) error {
	ns := namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}
	if err := p.clientSet.CoreV1().Secrets(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deleting repository host secret %s/%s: %w", ns, name, err)
	}
	return nil
}

func secretToRepositoryHost(secret *corev1.Secret) *RepositoryHost {
	data := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = string(v)
	}

	baseURL, ok := data["BASE_URL"]
	if !ok || baseURL == "" {
		return nil
	}

	return &RepositoryHost{
		Name:        secret.Name,
		Namespace:   secret.Namespace,
		Type:        InferHostType(baseURL),
		BaseURL:     baseURL,
		Credentials: data,
	}
}
