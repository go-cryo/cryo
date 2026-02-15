package backupjob

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"gopkg.in/yaml.v3"
)

const labelSelector = "go-cryo.github.com/config=true"

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

func (p *KubernetesProvider) List(ctx context.Context) ([]*BackupJob, error) {
	configMaps, err := p.clientSet.CoreV1().ConfigMaps(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing backup job config maps: %w", err)
	}

	jobs := make([]*BackupJob, 0, len(configMaps.Items))
	for _, cm := range configMaps.Items {
		job, err := configMapToBackupJob(&cm)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (p *KubernetesProvider) Get(ctx context.Context, namespace, name string) (*BackupJob, error) {
	ns := namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}
	cm, err := p.clientSet.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting backup job config map %s/%s: %w", ns, name, err)
	}

	return configMapToBackupJob(cm)
}

func (p *KubernetesProvider) Create(ctx context.Context, req *CreateBackupJobRequest) (*BackupJob, error) {
	ns := req.Namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}

	if req.PSQL != nil {
		if err := p.ensurePSQLCredentialSecret(ctx, ns, req.Name, req.PSQL); err != nil {
			return nil, err
		}
	}
	if req.S3 != nil {
		if err := p.ensureS3CredentialSecret(ctx, ns, req.Name, req.S3); err != nil {
			return nil, err
		}
	}

	cfg := &BackupJobConfig{
		Type:          req.Type,
		Schedule:      req.Schedule,
		Suspend:       req.Suspend,
		RepositoryRef: req.RepositoryRef,
		Image:         req.Image,
		Retention:     req.Retention,
		PSQL:          req.PSQL,
		S3:            req.S3,
		PVC:           req.PVC,
	}

	configData, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling backup job config: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: ns,
			Labels: map[string]string{
				"go-cryo.github.com/config": "true",
			},
		},
		Data: map[string]string{
			"config": string(configData),
		},
	}

	created, err := p.clientSet.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating backup job config map %s/%s: %w", ns, req.Name, err)
	}

	return configMapToBackupJob(created)
}

func (p *KubernetesProvider) Update(ctx context.Context, namespace, name string, req *UpdateBackupJobRequest) (*BackupJob, error) {
	ns := namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}

	cm, err := p.clientSet.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting backup job config map %s/%s: %w", ns, name, err)
	}

	cfg, err := ParseConfig(cm.Data["config"])
	if err != nil {
		return nil, fmt.Errorf("parsing existing backup job config %s/%s: %w", ns, name, err)
	}

	if req.Schedule != "" {
		cfg.Schedule = req.Schedule
	}
	if req.Suspend != nil {
		cfg.Suspend = *req.Suspend
	}
	if req.RepositoryRef != "" {
		cfg.RepositoryRef = req.RepositoryRef
	}
	if req.Image != "" {
		cfg.Image = req.Image
	}
	if req.Retention != nil {
		cfg.Retention = req.Retention
	}
	if req.PSQL != nil {
		if err := p.ensurePSQLCredentialSecret(ctx, ns, name, req.PSQL); err != nil {
			return nil, err
		}
		cfg.PSQL = req.PSQL
	}
	if req.S3 != nil {
		if err := p.ensureS3CredentialSecret(ctx, ns, name, req.S3); err != nil {
			return nil, err
		}
		cfg.S3 = req.S3
	}
	if req.PVC != nil {
		cfg.PVC = req.PVC
	}

	configData, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling backup job config: %w", err)
	}

	cm.Data["config"] = string(configData)

	updated, err := p.clientSet.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("updating backup job config map %s/%s: %w", ns, name, err)
	}

	return configMapToBackupJob(updated)
}

func (p *KubernetesProvider) Delete(ctx context.Context, namespace, name string) error {
	ns := namespace
	if ns == "" {
		ns = p.namespace
	}
	if ns == "" {
		ns = "default"
	}
	if err := p.clientSet.CoreV1().ConfigMaps(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deleting backup job config map %s/%s: %w", ns, name, err)
	}
	return nil
}

func (p *KubernetesProvider) ensurePSQLCredentialSecret(ctx context.Context, namespace, jobName string, cfg *PSQLConfig) error {
	secretName := jobName + "-psql-credentials"
	secretData := map[string][]byte{
		"hostname": []byte(cfg.Hostname),
		"username": []byte(cfg.Username),
		"database": []byte(cfg.Database),
	}
	if cfg.Port > 0 {
		secretData["port"] = []byte(fmt.Sprintf("%d", cfg.Port))
	}
	if cfg.Password != "" {
		secretData["password"] = []byte(cfg.Password)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"go-cryo.github.com/managed-by": "cryo",
				"go-cryo.github.com/backup-job": jobName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	existing, err := p.clientSet.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		// Preserve existing password if not provided in this update
		if cfg.Password == "" {
			if existingPw, ok := existing.Data["password"]; ok {
				secret.Data["password"] = existingPw
			}
		}
		existing.Data = secret.Data
		if _, err := p.clientSet.CoreV1().Secrets(namespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("updating psql credential secret %s/%s: %w", namespace, secretName, err)
		}
	} else {
		if _, err := p.clientSet.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("creating psql credential secret %s/%s: %w", namespace, secretName, err)
		}
	}

	cfg.CredentialSecretRef = secretName
	cfg.Password = ""
	return nil
}

func (p *KubernetesProvider) ensureS3CredentialSecret(ctx context.Context, namespace, jobName string, cfg *S3Config) error {
	if cfg.AccessKey == "" && cfg.SecretKey == "" {
		return nil
	}

	secretName := jobName + "-s3-credentials"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"go-cryo.github.com/managed-by": "cryo",
				"go-cryo.github.com/backup-job": jobName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"accessKey": []byte(cfg.AccessKey),
			"secretKey": []byte(cfg.SecretKey),
		},
	}

	existing, err := p.clientSet.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		existing.Data = secret.Data
		if _, err := p.clientSet.CoreV1().Secrets(namespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("updating s3 credential secret %s/%s: %w", namespace, secretName, err)
		}
	} else {
		if _, err := p.clientSet.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("creating s3 credential secret %s/%s: %w", namespace, secretName, err)
		}
	}

	cfg.CredentialsSecretRef = &S3CredentialRef{Name: secretName, AccessKeyKey: "accessKey", SecretKeyKey: "secretKey"}
	cfg.AccessKey = ""
	cfg.SecretKey = ""
	return nil
}

func configMapToBackupJob(cm *corev1.ConfigMap) (*BackupJob, error) {
	configData, ok := cm.Data["config"]
	if !ok || configData == "" {
		return nil, fmt.Errorf("config map %s/%s missing 'config' data key", cm.Namespace, cm.Name)
	}

	cfg, err := ParseConfig(configData)
	if err != nil {
		return nil, fmt.Errorf("parsing config from config map %s/%s: %w", cm.Namespace, cm.Name, err)
	}

	return &BackupJob{
		Name:          cm.Name,
		Namespace:     cm.Namespace,
		Type:          cfg.Type,
		Schedule:      cfg.Schedule,
		Suspend:       cfg.Suspend,
		RepositoryRef: cfg.RepositoryRef,
		Image:         cfg.Image,
		Retention:     cfg.Retention,
		PSQL:          cfg.PSQL,
		S3:            cfg.S3,
		PVC:           cfg.PVC,
	}, nil
}
