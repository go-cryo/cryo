package backupjob

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type BackupJobConfig struct {
	Type          BackupJobType    `yaml:"type"`
	Schedule      string           `yaml:"schedule"`
	Suspend       bool             `yaml:"suspend,omitempty"`
	RepositoryRef string           `yaml:"repositoryRef"`
	Image         string           `yaml:"image,omitempty"`
	Retention     *RetentionPolicy `yaml:"retention,omitempty"`
	PSQL          *PSQLConfig      `yaml:"psql,omitempty"`
	S3            *S3Config        `yaml:"s3,omitempty"`
	PVC           *PVCConfig       `yaml:"pvc,omitempty"`
}

func ParseConfig(data string) (*BackupJobConfig, error) {
	var cfg BackupJobConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		return nil, fmt.Errorf("parsing backup job config: %w", err)
	}

	if cfg.Type == "" {
		return nil, fmt.Errorf("backup job config missing 'type' field")
	}
	if cfg.Schedule == "" {
		return nil, fmt.Errorf("backup job config missing 'schedule' field")
	}
	if cfg.RepositoryRef == "" {
		return nil, fmt.Errorf("backup job config missing 'repositoryRef' field")
	}

	switch cfg.Type {
	case BackupJobTypePSQL:
		if cfg.PSQL == nil {
			return nil, fmt.Errorf("backup job type 'psql' requires 'psql' config block")
		}
	case BackupJobTypeS3:
		if cfg.S3 == nil {
			return nil, fmt.Errorf("backup job type 's3' requires 's3' config block")
		}
	case BackupJobTypePVC:
		if cfg.PVC == nil {
			return nil, fmt.Errorf("backup job type 'pvc' requires 'pvc' config block")
		}
	default:
		return nil, fmt.Errorf("unknown backup job type: %s", cfg.Type)
	}

	return &cfg, nil
}
