package restic

import "time"

type SnapshotSummary struct {
	TotalBytesProcessed uint64 `json:"total_bytes_processed"`
}

type Snapshot struct {
	ID       string           `json:"id"`
	Time     time.Time        `json:"time"`
	Hostname string           `json:"hostname"`
	Tags     []string         `json:"tags"`
	Paths    []string         `json:"paths"`
	Summary  *SnapshotSummary `json:"summary,omitempty"`
}

type RepositoryStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type SnapshotEntry struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Path  string `json:"path"`
	Size  uint64 `json:"size"`
	Mtime string `json:"mtime"`
}

type SnapshotBrowseResponse struct {
	SnapshotID string           `json:"snapshotId"`
	Path       string           `json:"path"`
	Entries    []*SnapshotEntry `json:"entries"`
}
