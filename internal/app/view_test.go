package app

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gkampitakis/go-snaps/snaps"

	"github.com/nateberkopec/2025-11-07-gogh/internal/githubclient"
	"github.com/nateberkopec/2025-11-07-gogh/internal/githuburl"
)

func TestViewSnapshot(t *testing.T) {
	client := stubGitHubClient{}
	m := New(Config{
		Client:       client,
		PollInterval: 0,
		BellEnabled:  true,
	})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	m = updated.(*Model)

	runs := []githubclient.WorkflowRun{
		{
			ID:           1,
			Name:         "unit",
			WorkflowName: "CI",
			RepoFullName: "example/api",
			Target:       "PR #12",
			TargetURL:    "https://github.com/example/api/pull/12",
			Status:       githubclient.RunStatusPending,
			StatusDetail: "in_progress",
		},
		{
			ID:            2,
			Name:          "lint",
			WorkflowName:  "CI",
			RepoFullName:  "example/web",
			Target:        "main",
			Status:        githubclient.RunStatusSuccess,
			LastUpdatedAt: time.Time{},
		},
	}

	m.absorbRuns(runs, githuburl.Parsed{Kind: githuburl.KindCommit, Owner: "example", Repo: "api"})

	snaps.MatchSnapshot(t, m.View())
}

type stubGitHubClient struct{}

func (stubGitHubClient) WorkflowRunByID(_ context.Context, _, _ string, _ int64) (githubclient.WorkflowRun, error) {
	return githubclient.WorkflowRun{}, nil
}

func (stubGitHubClient) RunsByPullRequest(_ context.Context, _, _ string, _ int) ([]githubclient.WorkflowRun, error) {
	return nil, nil
}

func (stubGitHubClient) RunsByCommit(_ context.Context, _, _, _ string) ([]githubclient.WorkflowRun, error) {
	return nil, nil
}
