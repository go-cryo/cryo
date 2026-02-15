package settings

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"gopkg.in/yaml.v3"
)

const (
	configMapName = "cryo-settings"
	dataKey       = "settings"
	labelKey      = "go-cryo.github.com/settings"
)

type KubernetesProvider struct {
	clientSet kubernetes.Interface
	namespace string
}

func NewKubernetesProvider(clientSet kubernetes.Interface, namespace string) *KubernetesProvider {
	if namespace == "" {
		namespace = "default"
	}
	return &KubernetesProvider{
		clientSet: clientSet,
		namespace: namespace,
	}
}

func (p *KubernetesProvider) Get(ctx context.Context) (*Settings, error) {
	cm, err := p.clientSet.CoreV1().ConfigMaps(p.namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return p.createDefault(ctx)
		}
		return nil, fmt.Errorf("getting settings configmap: %w", err)
	}

	return parseSettings(cm)
}

func (p *KubernetesProvider) Update(ctx context.Context, req *UpdateSettingsRequest) (*Settings, error) {
	current, err := p.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading current settings: %w", err)
	}

	if req.DefaultStorageClassName != nil {
		current.DefaultStorageClassName = *req.DefaultStorageClassName
	}
	if req.DefaultRetention != nil {
		current.DefaultRetention = req.DefaultRetention
	}
	if req.JobTTLSeconds != nil {
		current.JobTTLSeconds = *req.JobTTLSeconds
	}

	data, err := yaml.Marshal(current)
	if err != nil {
		return nil, fmt.Errorf("marshalling settings: %w", err)
	}

	cm, err := p.clientSet.CoreV1().ConfigMaps(p.namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting settings configmap for update: %w", err)
	}

	cm.Data[dataKey] = string(data)

	_, err = p.clientSet.CoreV1().ConfigMaps(p.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("updating settings configmap: %w", err)
	}

	log.Info().Msg("settings updated")
	return current, nil
}

func (p *KubernetesProvider) createDefault(ctx context.Context) (*Settings, error) {
	defaults := DefaultSettings()
	data, err := yaml.Marshal(defaults)
	if err != nil {
		return nil, fmt.Errorf("marshalling default settings: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: p.namespace,
			Labels: map[string]string{
				labelKey: "true",
			},
		},
		Data: map[string]string{
			dataKey: string(data),
		},
	}

	_, err = p.clientSet.CoreV1().ConfigMaps(p.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating default settings configmap: %w", err)
	}

	log.Info().Msg("created default settings configmap")
	return defaults, nil
}

func parseSettings(cm *corev1.ConfigMap) (*Settings, error) {
	raw, ok := cm.Data[dataKey]
	if !ok {
		return DefaultSettings(), nil
	}

	var s Settings
	if err := yaml.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("parsing settings yaml: %w", err)
	}

	if s.JobTTLSeconds == 0 {
		s.JobTTLSeconds = 604800
	}

	return &s, nil
}
