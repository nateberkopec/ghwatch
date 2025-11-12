package watch

import (
	"testing"
	"time"

	"github.com/nateberkopec/ghwatch/internal/githubclient"
	"github.com/nateberkopec/ghwatch/internal/githuburl"
)

func TestTrackerUpsertAndArchive(t *testing.T) {
	tracker := NewTracker()

	run := githubclient.WorkflowRun{
		ID:           1,
		Name:         "lint",
		WorkflowName: "CI",
		RepoFullName: "owner/repo",
		Status:       githubclient.RunStatusPending,
	}

	source := githuburl.Parsed{
		Kind:  githuburl.KindCommit,
		Owner: "owner",
		Repo:  "repo",
		SHA:   "abc123",
	}

	isNew, changed := tracker.Upsert(run, source)
	if !isNew || changed {
		t.Fatalf("expected new run without status change, got new=%v changed=%v", isNew, changed)
	}
	if tracker.LenActive() != 1 {
		t.Fatalf("expected 1 active run, got %d", tracker.LenActive())
	}

	run.Status = githubclient.RunStatusSuccess
	isNew, changed = tracker.Upsert(run, source)
	if isNew || !changed {
		t.Fatalf("expected existing run with status change, got new=%v changed=%v", isNew, changed)
	}

	if !tracker.Archive(run.ID) {
		t.Fatal("expected archive to succeed")
	}
	if tracker.LenActive() != 0 || tracker.LenArchived() != 1 {
		t.Fatalf("unexpected sizes: active=%d archived=%d", tracker.LenActive(), tracker.LenArchived())
	}

	if !tracker.Unarchive(run.ID) {
		t.Fatal("expected unarchive to succeed")
	}
	if tracker.LenActive() != 1 {
		t.Fatalf("expected 1 active after unarchive, got %d", tracker.LenActive())
	}
}

func TestTrackerUpsertRevivesArchivedRun(t *testing.T) {
	tracker := NewTracker()
	run := githubclient.WorkflowRun{
		ID:           42,
		Name:         "build",
		WorkflowName: "CI",
		RepoFullName: "owner/repo",
		Status:       githubclient.RunStatusPending,
	}
	tracker.Upsert(run, githuburl.Parsed{})
	tracker.Archive(run.ID)

	run.Status = githubclient.RunStatusSuccess
	isNew, changed := tracker.Upsert(run, githuburl.Parsed{Kind: githuburl.KindCommit, SHA: "abc"})

	if !isNew {
		t.Fatalf("expected archived run to be treated as new when re-added")
	}
	if !changed {
		t.Fatalf("expected status change to be detected")
	}
	if tracker.LenActive() != 1 || tracker.LenArchived() != 0 {
		t.Fatalf("expected run to move back to active: active=%d archived=%d", tracker.LenActive(), tracker.LenArchived())
	}
}
func TestTrackerVisibleRunsOrder(t *testing.T) {
	tracker := NewTracker()
	now := time.Now()
	for i := 0; i < 3; i++ {
		run := githubclient.WorkflowRun{
			ID:            int64(i + 1),
			Name:          "job",
			WorkflowName:  "CI",
			RepoFullName:  "owner/repo",
			Status:        githubclient.RunStatusPending,
			LastUpdatedAt: now,
		}
		tracker.Upsert(run, githuburl.Parsed{})
	}
	order := tracker.IDs(false)
	if len(order) != 3 || order[0] != 3 {
		t.Fatalf("expected newest run first, got %v", order)
	}
}
