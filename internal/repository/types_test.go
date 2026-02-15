package repository

import "testing"

func TestInferRepositoryType(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want RepositoryType
	}{
		{"s3 URL", "s3:https://bucket.s3.amazonaws.com/path", RepositoryTypeS3},
		{"sftp URL", "sftp:user@host:/path", RepositoryTypeSFTP},
		{"rest URL", "rest:https://backup.example.com/path", RepositoryTypeRest},
		{"local path", "/var/backups/restic", RepositoryTypeLocal},
		{"unknown URL", "https://example.com", RepositoryTypeLocal},
		{"empty string", "", RepositoryTypeLocal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferRepositoryType(tt.url); got != tt.want {
				t.Errorf("InferRepositoryType(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
