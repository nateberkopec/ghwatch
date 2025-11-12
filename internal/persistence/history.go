package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type historyData struct {
	Version  int       `json:"version"`
	Commands []string  `json:"commands"`
	SavedAt  time.Time `json:"saved_at"`
}

const historyVersion = 1
const maxHistorySize = 1000

func historyPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.json"), nil
}

func SaveHistory(commands []string) error {
	path, err := historyPath()
	if err != nil {
		return err
	}

	// Limit history size
	if len(commands) > maxHistorySize {
		commands = commands[len(commands)-maxHistorySize:]
	}

	history := historyData{
		Version:  historyVersion,
		Commands: commands,
		SavedAt:  time.Now(),
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
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

func LoadHistory() ([]string, error) {
	path, err := historyPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var history historyData
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to unmarshal history: %w", err)
	}

	if history.Version != historyVersion {
		return nil, fmt.Errorf("unsupported history version: %d", history.Version)
	}

	return history.Commands, nil
}
