package backupjob

import "context"

type Provider interface {
	List(ctx context.Context) ([]*BackupJob, error)
	Get(ctx context.Context, namespace, name string) (*BackupJob, error)
	Create(ctx context.Context, req *CreateBackupJobRequest) (*BackupJob, error)
	Update(ctx context.Context, namespace, name string, req *UpdateBackupJobRequest) (*BackupJob, error)
	Delete(ctx context.Context, namespace, name string) error
}

type CreateBackupJobRequest struct {
	Name          string           `json:"name"`
	Namespace     string           `json:"namespace"`
	Type          BackupJobType    `json:"type"`
	Schedule      string           `json:"schedule"`
	Suspend       bool             `json:"suspend"`
	RepositoryRef string           `json:"repositoryRef"`
	Image         string           `json:"image,omitempty"`
	Retention     *RetentionPolicy `json:"retention,omitempty"`
	PSQL          *PSQLConfig      `json:"psql,omitempty"`
	S3            *S3Config        `json:"s3,omitempty"`
	PVC           *PVCConfig       `json:"pvc,omitempty"`
}

type UpdateBackupJobRequest struct {
	Schedule      string           `json:"schedule,omitempty"`
	Suspend       *bool            `json:"suspend,omitempty"`
	RepositoryRef string           `json:"repositoryRef,omitempty"`
	Image         string           `json:"image,omitempty"`
	Retention     *RetentionPolicy `json:"retention,omitempty"`
	PSQL          *PSQLConfig      `json:"psql,omitempty"`
	S3            *S3Config        `json:"s3,omitempty"`
	PVC           *PVCConfig       `json:"pvc,omitempty"`
}
