package watch

import (
	"slices"
	"time"

	"github.com/nateberkopec/2025-11-07-gogh/internal/githubclient"
	"github.com/nateberkopec/2025-11-07-gogh/internal/githuburl"
)

// Tracker keeps the catalog of workflow runs that the UI renders.
type Tracker struct {
	activeOrder   []int64
	archivedOrder []int64
	active        map[int64]*TrackedRun
	archived      map[int64]*TrackedRun
}

// TrackedRun records metadata about a workflow run along with its current state.
type TrackedRun struct {
	Run        githubclient.WorkflowRun
	Source     githuburl.Parsed
	AddedAt    time.Time
	ArchivedAt time.Time
}

// ExportState returns a snapshot of the tracker state for persistence.
func (t *Tracker) ExportState() (active []*TrackedRun, activeOrder []int64, archived []*TrackedRun, archivedOrder []int64) {
	activeCopy := make([]*TrackedRun, 0, len(t.active))
	for _, run := range t.active {
		activeCopy = append(activeCopy, run)
	}

	archivedCopy := make([]*TrackedRun, 0, len(t.archived))
	for _, run := range t.archived {
		archivedCopy = append(archivedCopy, run)
	}

	return activeCopy, slices.Clone(t.activeOrder), archivedCopy, slices.Clone(t.archivedOrder)
}

// ImportState restores tracker state from persistence data.
func (t *Tracker) ImportState(active []*TrackedRun, activeOrder []int64, archived []*TrackedRun, archivedOrder []int64) {
	t.active = make(map[int64]*TrackedRun, len(active))
	for _, run := range active {
		t.active[run.Run.ID] = run
	}

	t.archived = make(map[int64]*TrackedRun, len(archived))
	for _, run := range archived {
		t.archived[run.Run.ID] = run
	}

	t.activeOrder = slices.Clone(activeOrder)
	t.archivedOrder = slices.Clone(archivedOrder)
}

// NewTracker creates a tracker with no runs.
func NewTracker() *Tracker {
	return &Tracker{
		active:   make(map[int64]*TrackedRun),
		archived: make(map[int64]*TrackedRun),
	}
}

// Upsert stores or refreshes a workflow run. It returns whether the run is new
// and whether its state changed during the update.
func (t *Tracker) Upsert(run githubclient.WorkflowRun, source githuburl.Parsed) (newRun bool, statusChanged bool) {
	if existing, ok := t.active[run.ID]; ok {
		statusChanged = existing.Run.Status != run.Status
		existing.Run = run
		if existing.Source.Kind == githuburl.KindUnknown && source.Kind != githuburl.KindUnknown {
			existing.Source = source
		}
		return false, statusChanged
	}

	if existing, ok := t.archived[run.ID]; ok {
		statusChanged = existing.Run.Status != run.Status
		existing.Run = run
		if existing.Source.Kind == githuburl.KindUnknown && source.Kind != githuburl.KindUnknown {
			existing.Source = source
		}
		delete(t.archived, run.ID)
		t.archivedOrder = removeID(t.archivedOrder, run.ID)
		t.active[run.ID] = existing
		t.activeOrder = prependUnique(t.activeOrder, run.ID)
		return true, statusChanged
	}

	entry := &TrackedRun{
		Run:     run,
		Source:  source,
		AddedAt: time.Now(),
	}
	t.active[run.ID] = entry
	t.activeOrder = prependUnique(t.activeOrder, run.ID)
	return true, false
}

// Archive moves a run out of the active list.
func (t *Tracker) Archive(id int64) bool {
	run, ok := t.active[id]
	if !ok {
		return false
	}
	delete(t.active, id)
	t.activeOrder = removeID(t.activeOrder, id)
	run.ArchivedAt = time.Now()
	t.archived[id] = run
	t.archivedOrder = prependUnique(t.archivedOrder, id)
	return true
}

// Unarchive moves a run back to the active list.
func (t *Tracker) Unarchive(id int64) bool {
	run, ok := t.archived[id]
	if !ok {
		return false
	}
	delete(t.archived, id)
	t.archivedOrder = removeID(t.archivedOrder, id)
	t.active[id] = run
	t.activeOrder = prependUnique(t.activeOrder, id)
	return true
}

// VisibleRuns returns the runs in display order.
func (t *Tracker) VisibleRuns(showArchived bool) []*TrackedRun {
	if showArchived {
		return collectRuns(t.archivedOrder, t.archived)
	}
	return collectRuns(t.activeOrder, t.active)
}

// IDs returns the IDs in display order.
func (t *Tracker) IDs(showArchived bool) []int64 {
	if showArchived {
		return slices.Clone(t.archivedOrder)
	}
	return slices.Clone(t.activeOrder)
}

// LenActive exposes the current active count.
func (t *Tracker) LenActive() int {
	return len(t.activeOrder)
}

// LenArchived exposes the archived count.
func (t *Tracker) LenArchived() int {
	return len(t.archivedOrder)
}

func collectRuns(order []int64, lookup map[int64]*TrackedRun) []*TrackedRun {
	items := make([]*TrackedRun, 0, len(order))
	for _, id := range order {
		if run, ok := lookup[id]; ok {
			items = append(items, run)
		}
	}
	return items
}

func prependUnique(items []int64, id int64) []int64 {
	items = removeID(items, id)
	return append([]int64{id}, items...)
}

func removeID(items []int64, id int64) []int64 {
	out := items[:0]
	for _, existing := range items {
		if existing == id {
			continue
		}
		out = append(out, existing)
	}
	return out
}
