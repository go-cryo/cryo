package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-cryo/cryo/internal/repositoryhost"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const labelSelector = "go-cryo.github.com/repository=true"

type KubernetesProvider struct {
	clientSet    kubernetes.Interface
	namespace    string
	hostProvider repositoryhost.Provider
}

func NewKubernetesProvider(clientSet kubernetes.Interface, namespace string, hostProvider repositoryhost.Provider) *KubernetesProvider {
	return &KubernetesProvider{
		clientSet:    clientSet,
		namespace:    namespace,
		hostProvider: hostProvider,
	}
}

func (p *KubernetesProvider) List(ctx context.Context) ([]*Repository, error) {
	secrets, err := p.clientSet.CoreV1().Secrets(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing repository secrets: %w", err)
	}

	repos := make([]*Repository, 0, len(secrets.Items))
	for _, secret := range secrets.Items {
		repo, err := p.resolveRepository(ctx, &secret)
		if err != nil {
			continue
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

func (p *KubernetesProvider) Get(ctx context.Context, namespace, name string) (*Repository, error) {
	ns := namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}
	secret, err := p.clientSet.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting repository secret %s/%s: %w", ns, name, err)
	}

	return p.resolveRepository(ctx, secret)
}

func (p *KubernetesProvider) Create(ctx context.Context, req *CreateRepositoryRequest) (*Repository, error) {
	ns := req.Namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}

	data := map[string][]byte{
		"HOST_REF":        []byte(req.HostRef),
		"PATH":            []byte(req.Path),
		"RESTIC_PASSWORD": []byte(req.ResticPassword),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: ns,
			Labels: map[string]string{
				"go-cryo.github.com/repository": "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	created, err := p.clientSet.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating repository secret %s/%s: %w", ns, req.Name, err)
	}

	return p.resolveRepository(ctx, created)
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
		return fmt.Errorf("deleting repository secret %s/%s: %w", ns, name, err)
	}
	return nil
}

func (p *KubernetesProvider) Update(ctx context.Context, namespace, name string, req *UpdateRepositoryRequest) (*Repository, error) {
	ns := namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}

	secret, err := p.clientSet.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting repository secret %s/%s: %w", ns, name, err)
	}

	if req.HostRef != "" {
		secret.Data["HOST_REF"] = []byte(req.HostRef)
	}
	if req.Path != "" {
		secret.Data["PATH"] = []byte(req.Path)
	}
	if req.ResticPassword != "" {
		secret.Data["RESTIC_PASSWORD"] = []byte(req.ResticPassword)
	}

	updated, err := p.clientSet.CoreV1().Secrets(ns).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("updating repository secret %s/%s: %w", ns, name, err)
	}

	return p.resolveRepository(ctx, updated)
}

func (p *KubernetesProvider) resolveRepository(ctx context.Context, secret *corev1.Secret) (*Repository, error) {
	data := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		data[k] = string(v)
	}

	hostRef := data["HOST_REF"]
	resticRepo := data["RESTIC_REPOSITORY"]

	if hostRef != "" {
		return p.resolveViaHost(ctx, secret, data)
	}
	if resticRepo != "" {
		return p.resolveLegacy(secret, data)
	}

	return nil, fmt.Errorf("secret %s/%s has neither HOST_REF nor RESTIC_REPOSITORY", secret.Namespace, secret.Name)
}

func (p *KubernetesProvider) resolveViaHost(ctx context.Context, secret *corev1.Secret, data map[string]string) (*Repository, error) {
	hostRef := data["HOST_REF"]
	path := data["PATH"]
	resticPassword := data["RESTIC_PASSWORD"]

	parts := strings.SplitN(hostRef, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid HOST_REF format %q in secret %s/%s", hostRef, secret.Namespace, secret.Name)
	}

	host, err := p.hostProvider.Get(ctx, parts[0], parts[1])
	if err != nil {
		return nil, fmt.Errorf("resolving host %s for repository %s/%s: %w", hostRef, secret.Namespace, secret.Name, err)
	}

	resolvedURL := resolveResticURL(host, path)

	creds := make(map[string]string)
	for k, v := range host.Credentials {
		creds[k] = v
	}
	creds["RESTIC_REPOSITORY"] = resolvedURL
	creds["RESTIC_PASSWORD"] = resticPassword

	return &Repository{
		Name:                secret.Name,
		Namespace:           secret.Namespace,
		Type:                InferRepositoryType(resolvedURL),
		URL:                 resolvedURL,
		HostRef:             hostRef,
		Path:                path,
		Credentials:         creds,
		HostSecretName:      parts[1],
		HostSecretNamespace: parts[0],
	}, nil
}

func (p *KubernetesProvider) resolveLegacy(secret *corev1.Secret, data map[string]string) (*Repository, error) {
	repoURL := data["RESTIC_REPOSITORY"]
	if repoURL == "" {
		return nil, fmt.Errorf("secret %s/%s is missing RESTIC_REPOSITORY", secret.Namespace, secret.Name)
	}

	return &Repository{
		Name:        secret.Name,
		Namespace:   secret.Namespace,
		Type:        InferRepositoryType(repoURL),
		URL:         repoURL,
		Credentials: data,
	}, nil
}

func resolveResticURL(host *repositoryhost.RepositoryHost, path string) string {
	baseURL := strings.TrimRight(host.BaseURL, "/")
	path = strings.TrimLeft(path, "/")

	if path == "" {
		return baseURL
	}

	switch host.Type {
	case repositoryhost.HostTypeSFTP:
		if strings.HasSuffix(baseURL, ":") {
			return baseURL + path
		}
		return baseURL + "/" + path
	default:
		return baseURL + "/" + path
	}
}
