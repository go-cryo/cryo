package settings

type RetentionPolicy struct {
	KeepLast    int `json:"keepLast,omitempty" yaml:"keepLast,omitempty"`
	KeepDaily   int `json:"keepDaily,omitempty" yaml:"keepDaily,omitempty"`
	KeepWeekly  int `json:"keepWeekly,omitempty" yaml:"keepWeekly,omitempty"`
	KeepMonthly int `json:"keepMonthly,omitempty" yaml:"keepMonthly,omitempty"`
}

type Settings struct {
	DefaultStorageClassName string           `json:"defaultStorageClassName" yaml:"defaultStorageClassName"`
	DefaultRetention        *RetentionPolicy `json:"defaultRetention,omitempty" yaml:"defaultRetention,omitempty"`
	JobTTLSeconds           int32            `json:"jobTTLSeconds" yaml:"jobTTLSeconds"`
}

func DefaultSettings() *Settings {
	return &Settings{
		DefaultStorageClassName: "",
		DefaultRetention:        nil,
		JobTTLSeconds:           604800,
	}
}
