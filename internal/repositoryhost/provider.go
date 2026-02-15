package repositoryhost

import "context"

type Provider interface {
	List(ctx context.Context) ([]*RepositoryHost, error)
	Get(ctx context.Context, namespace, name string) (*RepositoryHost, error)
	Create(ctx context.Context, req *CreateHostRequest) (*RepositoryHost, error)
	Update(ctx context.Context, namespace, name string, req *UpdateHostRequest) (*RepositoryHost, error)
	Delete(ctx context.Context, namespace, name string) error
}

type CreateHostRequest struct {
	Name               string `json:"name"`
	Namespace          string `json:"namespace"`
	BaseURL            string `json:"baseUrl"`
	AwsAccessKeyID     string `json:"awsAccessKeyId,omitempty"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey,omitempty"`
	AwsDefaultRegion   string `json:"awsDefaultRegion,omitempty"`
}

type UpdateHostRequest struct {
	BaseURL            string `json:"baseUrl"`
	AwsAccessKeyID     string `json:"awsAccessKeyId,omitempty"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey,omitempty"`
	AwsDefaultRegion   string `json:"awsDefaultRegion,omitempty"`
}
