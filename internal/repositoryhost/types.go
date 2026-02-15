package repositoryhost

import "strings"

type HostType string

const (
	HostTypeS3    HostType = "s3"
	HostTypeLocal HostType = "local"
	HostTypeSFTP  HostType = "sftp"
	HostTypeRest  HostType = "rest"
)

type RepositoryHost struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Type        HostType          `json:"type"`
	BaseURL     string            `json:"baseUrl"`
	Credentials map[string]string `json:"-"`
}

func InferHostType(url string) HostType {
	switch {
	case strings.HasPrefix(url, "s3:"):
		return HostTypeS3
	case strings.HasPrefix(url, "sftp:"):
		return HostTypeSFTP
	case strings.HasPrefix(url, "rest:"):
		return HostTypeRest
	default:
		return HostTypeLocal
	}
}
