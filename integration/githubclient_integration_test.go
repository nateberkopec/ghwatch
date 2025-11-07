//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nateberkopec/2025-11-07-gogh/internal/githubclient"
)

func TestGitHubClientRunsByCommit(t *testing.T) {
	owner := "vercel"
	repo := "next.js"

	client := githubclient.New("")

	branch := fetchDefaultBranch(t, owner, repo)
	sha := fetchBranchHeadSHA(t, owner, repo, branch)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runs, err := client.RunsByCommit(ctx, owner, repo, sha)
	if err != nil {
		t.Fatalf("RunsByCommit returned error: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected workflow runs for commit %s on %s/%s", sha, owner, repo)
	}

	run := runs[0]
	refreshed, err := client.WorkflowRunByID(ctx, owner, repo, run.ID)
	if err != nil {
		t.Fatalf("WorkflowRunByID returned error: %v", err)
	}
	if refreshed.ID != run.ID {
		t.Fatalf("expected same run ID, got %d vs %d", refreshed.ID, run.ID)
	}
	if refreshed.RepoFullName == "" || refreshed.Name == "" {
		t.Fatalf("expected repo/name fields to be populated: %#v", refreshed)
	}
}

func fetchDefaultBranch(t *testing.T, owner, repo string) string {
	t.Helper()
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	var payload struct {
		DefaultBranch string `json:"default_branch"`
	}
	requestJSON(t, url, &payload)
	if payload.DefaultBranch == "" {
		t.Fatalf("default_branch empty for %s/%s", owner, repo)
	}
	return payload.DefaultBranch
}

func fetchBranchHeadSHA(t *testing.T, owner, repo, branch string) string {
	t.Helper()
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, branch)
	var payload struct {
		SHA string `json:"sha"`
	}
	requestJSON(t, url, &payload)
	if payload.SHA == "" {
		t.Fatalf("head sha empty for %s/%s@%s", owner, repo, branch)
	}
	return payload.SHA
}

func requestJSON(t *testing.T, url string, v any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gogh-watcher-tests")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d for %s", res.StatusCode, url)
	}
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
}
