package settings

import "context"

type UpdateSettingsRequest struct {
	DefaultStorageClassName *string          `json:"defaultStorageClassName,omitempty"`
	DefaultRetention        *RetentionPolicy `json:"defaultRetention,omitempty"`
	JobTTLSeconds           *int32           `json:"jobTTLSeconds,omitempty"`
}

type Provider interface {
	Get(ctx context.Context) (*Settings, error)
	Update(ctx context.Context, req *UpdateSettingsRequest) (*Settings, error)
}
