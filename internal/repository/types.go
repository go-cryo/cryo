package repository

import "strings"

type RepositoryType string

const (
	RepositoryTypeS3    RepositoryType = "s3"
	RepositoryTypeLocal RepositoryType = "local"
	RepositoryTypeSFTP  RepositoryType = "sftp"
	RepositoryTypeRest  RepositoryType = "rest"
)

type Repository struct {
	Name                string            `json:"name"`
	Namespace           string            `json:"namespace"`
	Type                RepositoryType    `json:"type"`
	URL                 string            `json:"url"`
	HostRef             string            `json:"hostRef,omitempty"`
	Path                string            `json:"path,omitempty"`
	Credentials         map[string]string `json:"-"`
	HostSecretName      string            `json:"-"`
	HostSecretNamespace string            `json:"-"`
}

func InferRepositoryType(url string) RepositoryType {
	switch {
	case strings.HasPrefix(url, "s3:"):
		return RepositoryTypeS3
	case strings.HasPrefix(url, "sftp:"):
		return RepositoryTypeSFTP
	case strings.HasPrefix(url, "rest:"):
		return RepositoryTypeRest
	default:
		return RepositoryTypeLocal
	}
}
