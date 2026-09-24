package executor

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-cryo/cryo/internal/backupjob"
)

func TestStagingAccessMode(t *testing.T) {
	cases := []struct {
		in      string
		want    corev1.PersistentVolumeAccessMode
		wantErr bool
	}{
		{"", corev1.ReadWriteOnce, false},
		{"ReadOnlyMany", corev1.ReadOnlyMany, false},
		{"ReadWriteMany", corev1.ReadWriteMany, false},
		{"readonlymany", "", true},
	}
	for _, c := range cases {
		got, err := stagingAccessMode(&backupjob.PVCConfig{StagingAccessMode: c.in})
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("stagingAccessMode(%q) = %q, %v; want %q, err=%v", c.in, got, err, c.want, c.wantErr)
		}
	}
}
