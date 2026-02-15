package repository

import "context"

type Provider interface {
	List(ctx context.Context) ([]*Repository, error)
	Get(ctx context.Context, namespace, name string) (*Repository, error)
	Create(ctx context.Context, repo *CreateRepositoryRequest) (*Repository, error)
	Update(ctx context.Context, namespace, name string, req *UpdateRepositoryRequest) (*Repository, error)
	Delete(ctx context.Context, namespace, name string) error
}

type CreateRepositoryRequest struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	HostRef        string `json:"hostRef"`
	Path           string `json:"path"`
	ResticPassword string `json:"resticPassword"`
}

type UpdateRepositoryRequest struct {
	HostRef        string `json:"hostRef"`
	Path           string `json:"path"`
	ResticPassword string `json:"resticPassword"`
}
