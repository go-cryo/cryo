package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/go-cryo/cryo/internal/repository"
	"github.com/rs/zerolog/log"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Init(ctx context.Context, repo *repository.Repository) error {
	log.Debug().Str("repository", repo.Name).Msg("initializing restic repository")

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "restic", "init")
	cmd.Env = buildEnv(repo)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restic init failed: %s", stderr.String())
	}

	return nil
}

func (s *Service) Check(ctx context.Context, repo *repository.Repository) (*RepositoryStatus, error) {
	log.Debug().Str("repository", repo.Name).Msg("checking restic repository")

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "restic", "check", "--no-lock")
	cmd.Env = buildEnv(repo)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &RepositoryStatus{
			OK:      false,
			Message: stderr.String(),
		}, nil
	}

	return &RepositoryStatus{OK: true}, nil
}

func (s *Service) ListSnapshots(ctx context.Context, repo *repository.Repository) ([]*Snapshot, error) {
	log.Debug().Str("repository", repo.Name).Msg("listing restic snapshots")

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "restic", "snapshots", "--json", "--no-lock")
	cmd.Env = buildEnv(repo)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("restic snapshots failed: %s", stderr.String())
	}

	var snapshots []*Snapshot
	if err := json.Unmarshal(stdout.Bytes(), &snapshots); err != nil {
		return nil, fmt.Errorf("parsing restic snapshots output: %w", err)
	}

	return snapshots, nil
}

func (s *Service) ListSnapshotFiles(ctx context.Context, repo *repository.Repository, snapshotID string, browsePath string) (*SnapshotBrowseResponse, error) {
	log.Debug().Str("repository", repo.Name).Str("snapshot", snapshotID).Str("path", browsePath).Msg("browsing snapshot files")

	// Normalize browse path
	if browsePath == "" {
		browsePath = "/"
	}
	browsePath = path.Clean(browsePath)
	if !strings.HasPrefix(browsePath, "/") {
		browsePath = "/" + browsePath
	}

	cmd := exec.CommandContext(ctx, "restic", "ls", "--json", "--no-lock", snapshotID)
	cmd.Env = buildEnv(repo)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting restic ls: %w", err)
	}

	seen := make(map[string]bool)
	var entries []*SnapshotEntry
	scanner := bufio.NewScanner(stdout)
	firstLine := true

	for scanner.Scan() {
		line := scanner.Bytes()

		// Skip the first JSON line which is snapshot metadata (no "type" field)
		if firstLine {
			firstLine = false
			continue
		}

		var raw struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Path  string `json:"path"`
			Size  uint64 `json:"size"`
			Mtime string `json:"mtime"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		// Filter to direct children of the browse path
		entryDir := path.Dir(raw.Path)
		if entryDir != browsePath {
			continue
		}

		// Deduplicate
		if seen[raw.Path] {
			continue
		}
		seen[raw.Path] = true

		entries = append(entries, &SnapshotEntry{
			Name:  raw.Name,
			Type:  raw.Type,
			Path:  raw.Path,
			Size:  raw.Size,
			Mtime: raw.Mtime,
		})
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("restic ls failed: %s", stderr.String())
	}

	return &SnapshotBrowseResponse{
		SnapshotID: snapshotID,
		Path:       browsePath,
		Entries:    entries,
	}, nil
}

func buildEnv(repo *repository.Repository) []string {
	env := os.Environ()
	for k, v := range repo.Credentials {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
