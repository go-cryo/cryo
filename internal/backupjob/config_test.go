package backupjob

import (
	"strings"
	"testing"
)

func TestParseConfig_ValidPSQL(t *testing.T) {
	yaml := `
type: psql
schedule: "0 2 * * *"
repositoryRef: default/my-repo
psql:
  hostname: db.example.com
  port: 5432
  username: admin
  database: mydb
`
	cfg, err := ParseConfig(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Type != BackupJobTypePSQL {
		t.Errorf("Type = %q, want %q", cfg.Type, BackupJobTypePSQL)
	}
	if cfg.Schedule != "0 2 * * *" {
		t.Errorf("Schedule = %q, want %q", cfg.Schedule, "0 2 * * *")
	}
	if cfg.RepositoryRef != "default/my-repo" {
		t.Errorf("RepositoryRef = %q, want %q", cfg.RepositoryRef, "default/my-repo")
	}
	if cfg.PSQL == nil {
		t.Fatal("PSQL config is nil")
	}
	if cfg.PSQL.Hostname != "db.example.com" {
		t.Errorf("PSQL.Hostname = %q, want %q", cfg.PSQL.Hostname, "db.example.com")
	}
	if cfg.PSQL.Port != 5432 {
		t.Errorf("PSQL.Port = %d, want %d", cfg.PSQL.Port, 5432)
	}
}

func TestParseConfig_ValidS3(t *testing.T) {
	yaml := `
type: s3
schedule: "0 3 * * *"
repositoryRef: default/my-repo
s3:
  endpoint: s3.amazonaws.com
  bucket: my-bucket
`
	cfg, err := ParseConfig(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Type != BackupJobTypeS3 {
		t.Errorf("Type = %q, want %q", cfg.Type, BackupJobTypeS3)
	}
	if cfg.S3 == nil {
		t.Fatal("S3 config is nil")
	}
	if cfg.S3.Endpoint != "s3.amazonaws.com" {
		t.Errorf("S3.Endpoint = %q, want %q", cfg.S3.Endpoint, "s3.amazonaws.com")
	}
	if cfg.S3.Bucket != "my-bucket" {
		t.Errorf("S3.Bucket = %q, want %q", cfg.S3.Bucket, "my-bucket")
	}
}

func TestParseConfig_ValidPVC(t *testing.T) {
	yaml := `
type: pvc
schedule: "0 4 * * *"
repositoryRef: default/my-repo
pvc:
  claimName: my-pvc
  volumeSnapshotClassName: csi-snapclass
  snapshotRetention: 3
`
	cfg, err := ParseConfig(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Type != BackupJobTypePVC {
		t.Errorf("Type = %q, want %q", cfg.Type, BackupJobTypePVC)
	}
	if cfg.PVC == nil {
		t.Fatal("PVC config is nil")
	}
	if cfg.PVC.ClaimName != "my-pvc" {
		t.Errorf("PVC.ClaimName = %q, want %q", cfg.PVC.ClaimName, "my-pvc")
	}
	if cfg.PVC.VolumeSnapshotClassName != "csi-snapclass" {
		t.Errorf("PVC.VolumeSnapshotClassName = %q, want %q", cfg.PVC.VolumeSnapshotClassName, "csi-snapclass")
	}
	if cfg.PVC.SnapshotRetention != 3 {
		t.Errorf("PVC.SnapshotRetention = %d, want %d", cfg.PVC.SnapshotRetention, 3)
	}
}

func TestParseConfig_MissingType(t *testing.T) {
	yaml := `
schedule: "0 2 * * *"
repositoryRef: default/my-repo
psql:
  hostname: db.example.com
`
	_, err := ParseConfig(yaml)
	if err == nil {
		t.Fatal("expected error for missing type")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Errorf("error %q should mention 'type'", err.Error())
	}
}

func TestParseConfig_MissingSchedule(t *testing.T) {
	yaml := `
type: psql
repositoryRef: default/my-repo
psql:
  hostname: db.example.com
`
	_, err := ParseConfig(yaml)
	if err == nil {
		t.Fatal("expected error for missing schedule")
	}
	if !strings.Contains(err.Error(), "schedule") {
		t.Errorf("error %q should mention 'schedule'", err.Error())
	}
}

func TestParseConfig_MissingRepositoryRef(t *testing.T) {
	yaml := `
type: psql
schedule: "0 2 * * *"
psql:
  hostname: db.example.com
`
	_, err := ParseConfig(yaml)
	if err == nil {
		t.Fatal("expected error for missing repositoryRef")
	}
	if !strings.Contains(err.Error(), "repositoryRef") {
		t.Errorf("error %q should mention 'repositoryRef'", err.Error())
	}
}

func TestParseConfig_PSQLWithoutBlock(t *testing.T) {
	yaml := `
type: psql
schedule: "0 2 * * *"
repositoryRef: default/my-repo
`
	_, err := ParseConfig(yaml)
	if err == nil {
		t.Fatal("expected error for psql type without psql block")
	}
	if !strings.Contains(err.Error(), "psql") {
		t.Errorf("error %q should mention 'psql'", err.Error())
	}
}

func TestParseConfig_S3WithoutBlock(t *testing.T) {
	yaml := `
type: s3
schedule: "0 2 * * *"
repositoryRef: default/my-repo
`
	_, err := ParseConfig(yaml)
	if err == nil {
		t.Fatal("expected error for s3 type without s3 block")
	}
	if !strings.Contains(err.Error(), "s3") {
		t.Errorf("error %q should mention 's3'", err.Error())
	}
}

func TestParseConfig_PVCWithoutBlock(t *testing.T) {
	yaml := `
type: pvc
schedule: "0 2 * * *"
repositoryRef: default/my-repo
`
	_, err := ParseConfig(yaml)
	if err == nil {
		t.Fatal("expected error for pvc type without pvc block")
	}
	if !strings.Contains(err.Error(), "pvc") {
		t.Errorf("error %q should mention 'pvc'", err.Error())
	}
}

func TestParseConfig_UnknownType(t *testing.T) {
	yaml := `
type: mysql
schedule: "0 2 * * *"
repositoryRef: default/my-repo
`
	_, err := ParseConfig(yaml)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error %q should mention 'unknown'", err.Error())
	}
}

func TestParseConfig_InvalidYAML(t *testing.T) {
	yaml := `
type: psql
  bad indent: [
`
	_, err := ParseConfig(yaml)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseConfig_RetentionPolicy(t *testing.T) {
	yaml := `
type: psql
schedule: "0 2 * * *"
repositoryRef: default/my-repo
retention:
  keepLast: 5
  keepDaily: 7
  keepWeekly: 4
  keepMonthly: 12
psql:
  hostname: db.example.com
  username: admin
  database: mydb
`
	cfg, err := ParseConfig(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Retention == nil {
		t.Fatal("Retention is nil")
	}
	if cfg.Retention.KeepLast != 5 {
		t.Errorf("Retention.KeepLast = %d, want %d", cfg.Retention.KeepLast, 5)
	}
	if cfg.Retention.KeepDaily != 7 {
		t.Errorf("Retention.KeepDaily = %d, want %d", cfg.Retention.KeepDaily, 7)
	}
	if cfg.Retention.KeepWeekly != 4 {
		t.Errorf("Retention.KeepWeekly = %d, want %d", cfg.Retention.KeepWeekly, 4)
	}
	if cfg.Retention.KeepMonthly != 12 {
		t.Errorf("Retention.KeepMonthly = %d, want %d", cfg.Retention.KeepMonthly, 12)
	}
}

func TestParseConfig_SuspendAndImage(t *testing.T) {
	yaml := `
type: psql
schedule: "0 2 * * *"
repositoryRef: default/my-repo
suspend: true
image: custom-image:latest
psql:
  hostname: db.example.com
  username: admin
  database: mydb
`
	cfg, err := ParseConfig(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Suspend {
		t.Error("Suspend should be true")
	}
	if cfg.Image != "custom-image:latest" {
		t.Errorf("Image = %q, want %q", cfg.Image, "custom-image:latest")
	}
}
