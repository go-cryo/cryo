package repositoryhost

import "testing"

func TestInferHostType(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want HostType
	}{
		{"s3 URL", "s3:https://bucket.s3.amazonaws.com/path", HostTypeS3},
		{"sftp URL", "sftp:user@host:/path", HostTypeSFTP},
		{"rest URL", "rest:https://backup.example.com/path", HostTypeRest},
		{"local path", "/var/backups/restic", HostTypeLocal},
		{"unknown URL", "https://example.com", HostTypeLocal},
		{"empty string", "", HostTypeLocal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferHostType(tt.url); got != tt.want {
				t.Errorf("InferHostType(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
