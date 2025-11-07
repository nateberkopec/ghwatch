package persistence

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nateberkopec/2025-11-07-gogh/internal/githubclient"
	"github.com/nateberkopec/2025-11-07-gogh/internal/githuburl"
	"github.com/nateberkopec/2025-11-07-gogh/internal/watch"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	tracker := watch.NewTracker()

	source := githuburl.Parsed{
		Kind:     githuburl.KindPullRequest,
		Owner:    "test",
		Repo:     "repo",
		PRNumber: 42,
	}

	run := githubclient.WorkflowRun{
		ID:           12345,
		Name:         "Test Run",
		WorkflowName: "CI",
		RepoFullName: "test/repo",
		Status:       githubclient.RunStatusPending,
		HeadBranch:   "main",
	}

	tracker.Upsert(run, source)

	if err := SaveTracker(tracker); err != nil {
		t.Fatalf("SaveTracker failed: %v", err)
	}

	loaded := watch.NewTracker()
	if err := LoadTracker(loaded); err != nil {
		t.Fatalf("LoadTracker failed: %v", err)
	}

	loadedRuns := loaded.VisibleRuns(false)
	if len(loadedRuns) != 1 {
		t.Fatalf("expected 1 run, got %d", len(loadedRuns))
	}

	if loadedRuns[0].Run.ID != run.ID {
		t.Errorf("expected run ID %d, got %d", run.ID, loadedRuns[0].Run.ID)
	}

	if loadedRuns[0].Source.PRNumber != source.PRNumber {
		t.Errorf("expected PR number %d, got %d", source.PRNumber, loadedRuns[0].Source.PRNumber)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	tracker := watch.NewTracker()
	if err := LoadTracker(tracker); err != nil {
		t.Fatalf("LoadTracker should not fail on missing file: %v", err)
	}

	if len(tracker.VisibleRuns(false)) != 0 {
		t.Error("expected empty tracker from missing state file")
	}
}

func TestArchivePreservation(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	tracker := watch.NewTracker()

	run := githubclient.WorkflowRun{
		ID:           12345,
		Name:         "Test Run",
		RepoFullName: "test/repo",
		Status:       githubclient.RunStatusSuccess,
	}

	tracker.Upsert(run, githuburl.Parsed{})
	tracker.Archive(run.ID)

	if err := SaveTracker(tracker); err != nil {
		t.Fatalf("SaveTracker failed: %v", err)
	}

	loaded := watch.NewTracker()
	if err := LoadTracker(loaded); err != nil {
		t.Fatalf("LoadTracker failed: %v", err)
	}

	if len(loaded.VisibleRuns(false)) != 0 {
		t.Error("expected no active runs")
	}

	archivedRuns := loaded.VisibleRuns(true)
	if len(archivedRuns) != 1 {
		t.Fatalf("expected 1 archived run, got %d", len(archivedRuns))
	}

	if archivedRuns[0].Run.ID != run.ID {
		t.Errorf("expected run ID %d, got %d", run.ID, archivedRuns[0].Run.ID)
	}
}

func TestOrderPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	tracker := watch.NewTracker()

	for i := int64(1); i <= 5; i++ {
		run := githubclient.WorkflowRun{
			ID:           i,
			Name:         "Run",
			RepoFullName: "test/repo",
			Status:       githubclient.RunStatusPending,
		}
		tracker.Upsert(run, githuburl.Parsed{})
	}

	if err := SaveTracker(tracker); err != nil {
		t.Fatalf("SaveTracker failed: %v", err)
	}

	loaded := watch.NewTracker()
	if err := LoadTracker(loaded); err != nil {
		t.Fatalf("LoadTracker failed: %v", err)
	}

	ids := loaded.IDs(false)
	if len(ids) != 5 {
		t.Fatalf("expected 5 runs, got %d", len(ids))
	}

	for i := 0; i < 5; i++ {
		expected := int64(5 - i)
		if ids[i] != expected {
			t.Errorf("expected ID %d at position %d, got %d", expected, i, ids[i])
		}
	}
}

func TestStatePath(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	path, err := statePath()
	if err != nil {
		t.Fatalf("statePath failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "gogh", "runs.json")
	if path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, path)
	}
}

func TestTimestampPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	tracker := watch.NewTracker()

	addedAt := time.Now().Add(-1 * time.Hour)
	run := githubclient.WorkflowRun{
		ID:           12345,
		Name:         "Test Run",
		RepoFullName: "test/repo",
		Status:       githubclient.RunStatusSuccess,
	}

	tracker.Upsert(run, githuburl.Parsed{})
	active, activeOrder, archived, archivedOrder := tracker.ExportState()
	active[0].AddedAt = addedAt
	tracker.ImportState(active, activeOrder, archived, archivedOrder)

	if err := SaveTracker(tracker); err != nil {
		t.Fatalf("SaveTracker failed: %v", err)
	}

	loaded := watch.NewTracker()
	if err := LoadTracker(loaded); err != nil {
		t.Fatalf("LoadTracker failed: %v", err)
	}

	loadedRuns := loaded.VisibleRuns(false)
	if len(loadedRuns) != 1 {
		t.Fatalf("expected 1 run, got %d", len(loadedRuns))
	}

	if !loadedRuns[0].AddedAt.Equal(addedAt) {
		t.Errorf("expected AddedAt %v, got %v", addedAt, loadedRuns[0].AddedAt)
	}
}
