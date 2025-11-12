package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nateberkopec/ghwatch/internal/githubclient"
	"github.com/nateberkopec/ghwatch/internal/githuburl"
	"github.com/nateberkopec/ghwatch/internal/watch"
)

type trackedRunData struct {
	Run        githubclient.WorkflowRun `json:"run"`
	Source     githuburl.Parsed         `json:"source"`
	AddedAt    time.Time                `json:"added_at"`
	ArchivedAt time.Time                `json:"archived_at"`
}

type stateData struct {
	Version       int              `json:"version"`
	Active        []trackedRunData `json:"active"`
	ActiveOrder   []int64          `json:"active_order"`
	Archived      []trackedRunData `json:"archived"`
	ArchivedOrder []int64          `json:"archived_order"`
	SavedAt       time.Time        `json:"saved_at"`
}

const stateVersion = 1

func dataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		xdgData = filepath.Join(home, ".local", "share")
	}

	dir := filepath.Join(xdgData, "ghwatch")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	return dir, nil
}

func statePath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "runs.json"), nil
}

func convertToData(runs []*watch.TrackedRun) []trackedRunData {
	data := make([]trackedRunData, 0, len(runs))
	for _, run := range runs {
		data = append(data, trackedRunData{
			Run:        run.Run,
			Source:     run.Source,
			AddedAt:    run.AddedAt,
			ArchivedAt: run.ArchivedAt,
		})
	}
	return data
}

func convertFromData(data []trackedRunData) []*watch.TrackedRun {
	runs := make([]*watch.TrackedRun, 0, len(data))
	for _, d := range data {
		runs = append(runs, &watch.TrackedRun{
			Run:        d.Run,
			Source:     d.Source,
			AddedAt:    d.AddedAt,
			ArchivedAt: d.ArchivedAt,
		})
	}
	return runs
}

func SaveTracker(tracker *watch.Tracker) error {
	active, activeOrder, archived, archivedOrder := tracker.ExportState()

	path, err := statePath()
	if err != nil {
		return err
	}

	state := stateData{
		Version:       stateVersion,
		Active:        convertToData(active),
		ActiveOrder:   activeOrder,
		Archived:      convertToData(archived),
		ArchivedOrder: archivedOrder,
		SavedAt:       time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func LoadTracker(tracker *watch.Tracker) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state stateData
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	if state.Version != stateVersion {
		return fmt.Errorf("unsupported state version: %d", state.Version)
	}

	tracker.ImportState(
		convertFromData(state.Active),
		state.ActiveOrder,
		convertFromData(state.Archived),
		state.ArchivedOrder,
	)

	return nil
}
