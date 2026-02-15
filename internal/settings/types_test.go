package settings

import "testing"

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if s == nil {
		t.Fatal("DefaultSettings() returned nil")
	}
	if s.DefaultStorageClassName != "" {
		t.Errorf("DefaultStorageClassName = %q, want empty string", s.DefaultStorageClassName)
	}
	if s.DefaultRetention != nil {
		t.Errorf("DefaultRetention = %v, want nil", s.DefaultRetention)
	}
	if s.JobTTLSeconds != 604800 {
		t.Errorf("JobTTLSeconds = %d, want %d", s.JobTTLSeconds, 604800)
	}
}
