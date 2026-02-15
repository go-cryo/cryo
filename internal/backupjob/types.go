package backupjob

import "time"

type BackupJobType string

const (
	BackupJobTypePSQL BackupJobType = "psql"
	BackupJobTypeS3   BackupJobType = "s3"
	BackupJobTypePVC  BackupJobType = "pvc"
)

type BackupJob struct {
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
	LastRun       *BackupRun       `json:"lastRun,omitempty"`
	NextRun       *time.Time       `json:"nextRun,omitempty"`
}

type RetentionPolicy struct {
	KeepLast    int `json:"keepLast,omitempty" yaml:"keepLast,omitempty"`
	KeepDaily   int `json:"keepDaily,omitempty" yaml:"keepDaily,omitempty"`
	KeepWeekly  int `json:"keepWeekly,omitempty" yaml:"keepWeekly,omitempty"`
	KeepMonthly int `json:"keepMonthly,omitempty" yaml:"keepMonthly,omitempty"`
}

type PSQLConfig struct {
	Hostname                string `json:"hostname" yaml:"hostname"`
	Port                    int    `json:"port,omitempty" yaml:"port,omitempty"`
	Username                string `json:"username" yaml:"username"`
	Database                string `json:"database" yaml:"database"`
	Password                string `json:"password,omitempty" yaml:"-"`
	CredentialSecretRef     string `json:"credentialSecretRef,omitempty" yaml:"credentialSecretRef,omitempty"`
	StagingSize             string `json:"stagingSize,omitempty" yaml:"stagingSize,omitempty"`
	StagingStorageClassName string `json:"stagingStorageClassName,omitempty" yaml:"stagingStorageClassName,omitempty"`
}

type S3Config struct {
	Endpoint                string           `json:"endpoint" yaml:"endpoint"`
	Bucket                  string           `json:"bucket" yaml:"bucket"`
	CredentialsSecretRef    *S3CredentialRef `json:"credentialsSecretRef,omitempty" yaml:"credentialsSecretRef,omitempty"`
	AccessKey               string           `json:"accessKey,omitempty" yaml:"-"`
	SecretKey               string           `json:"secretKey,omitempty" yaml:"-"`
	StagingSize             string           `json:"stagingSize,omitempty" yaml:"stagingSize,omitempty"`
	StagingStorageClassName string           `json:"stagingStorageClassName,omitempty" yaml:"stagingStorageClassName,omitempty"`
}

type PVCConfig struct {
	ClaimName               string `json:"claimName" yaml:"claimName"`
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName,omitempty" yaml:"volumeSnapshotClassName,omitempty"`
	SnapshotRetention       int    `json:"snapshotRetention,omitempty" yaml:"snapshotRetention,omitempty"`
}

type SecretKeyRef struct {
	Name string `json:"name" yaml:"name"`
	Key  string `json:"key" yaml:"key"`
}

type S3CredentialRef struct {
	Name         string `json:"name" yaml:"name"`
	AccessKeyKey string `json:"accessKeyKey" yaml:"accessKeyKey"`
	SecretKeyKey string `json:"secretKeyKey" yaml:"secretKeyKey"`
}

type BackupRunStatus string

const (
	BackupRunStatusRunning   BackupRunStatus = "Running"
	BackupRunStatusSucceeded BackupRunStatus = "Succeeded"
	BackupRunStatusFailed    BackupRunStatus = "Failed"
)

type BackupRun struct {
	JobName   string          `json:"jobName"`
	Namespace string          `json:"namespace"`
	Name      string          `json:"name"`
	Status    BackupRunStatus `json:"status"`
	StartTime *time.Time      `json:"startTime,omitempty"`
	EndTime   *time.Time      `json:"endTime,omitempty"`
	Message   string          `json:"message,omitempty"`
}
