package githubclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// RunStatus represents the simplified workflow state that the UI displays.
type RunStatus string

const (
	RunStatusPending RunStatus = "pending"
	RunStatusSuccess RunStatus = "success"
	RunStatusFailed  RunStatus = "failed"
)

// WorkflowRun contains the normalized subset of GitHub workflow run data that
// the watcher needs to render UI and detect state changes.
type WorkflowRun struct {
	ID            int64
	Name          string
	WorkflowName  string
	RepoFullName  string
	Target        string
	TargetURL     string
	Status        RunStatus
	StatusDetail  string
	HTMLURL       string
	HeadBranch    string
	HeadSHA       string
	Event         string
	PRNumber      int
	PRURL         string
	LastUpdatedAt time.Time
}

// Client talks to the GitHub REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// New creates a GitHub client. If token is empty, well-known environment
// variables are checked (GITHUB_TOKEN, GH_TOKEN, GH_PAT).
func New(token string) *Client {
	if token == "" {
		token = firstNonEmpty(
			os.Getenv("GITHUB_TOKEN"),
			os.Getenv("GH_TOKEN"),
			os.Getenv("GH_PAT"),
		)
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		baseURL: "https://api.github.com",
		token:   token,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// WorkflowRunByID fetches a single workflow run.
func (c *Client) WorkflowRunByID(ctx context.Context, owner, repo string, runID int64) (WorkflowRun, error) {
	var payload workflowRunPayload
	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID)
	if err := c.getJSON(ctx, path, nil, &payload); err != nil {
		return WorkflowRun{}, err
	}
	run := convertRun(payload)
	if run.RepoFullName == "" {
		run.RepoFullName = fmt.Sprintf("%s/%s", owner, repo)
	}
	if pr := firstRunPullRequest(payload.PullRequests); pr != nil {
		run.Target = fmt.Sprintf("PR #%d", pr.Number)
		run.TargetURL = pr.HTMLURL
		run.PRNumber = pr.Number
		run.PRURL = pr.HTMLURL
	}
	return run, nil
}

// RunsByCommit fetches all runs matching the supplied commit SHA.
func (c *Client) RunsByCommit(ctx context.Context, owner, repo, sha string) ([]WorkflowRun, error) {
	query := map[string]string{"per_page": "30", "head_sha": sha}
	payload, err := c.listRuns(ctx, owner, repo, query)
	if err != nil {
		return nil, err
	}

	commitURL := fmt.Sprintf("https://github.com/%s/%s/commit/%s", owner, repo, sha)
	return decorateRuns(payload, func(r *WorkflowRun) {
		r.Target = fmt.Sprintf("commit %.7s", sha)
		if r.TargetURL == "" {
			r.TargetURL = commitURL
		}
	}), nil
}

// RunsByPullRequest finds the pull request, grabs the head SHA, and returns the
// associated workflow runs.
func (c *Client) RunsByPullRequest(ctx context.Context, owner, repo string, number int) ([]WorkflowRun, error) {
	prPath := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	var payload pullRequestPayload
	if err := c.getJSON(ctx, prPath, nil, &payload); err != nil {
		return nil, err
	}

	runs, err := c.RunsByCommit(ctx, owner, repo, payload.Head.SHA)
	if err != nil {
		return nil, err
	}

	prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, number)
	for i := range runs {
		runs[i].Target = fmt.Sprintf("PR #%d", number)
		runs[i].TargetURL = prURL
		runs[i].PRNumber = number
		runs[i].PRURL = prURL
	}

	return runs, nil
}

func (c *Client) listRuns(ctx context.Context, owner, repo string, query map[string]string) ([]workflowRunPayload, error) {
	path := fmt.Sprintf("/repos/%s/%s/actions/runs", owner, repo)
	var payload workflowRunsResponse
	if err := c.getJSON(ctx, path, query, &payload); err != nil {
		return nil, err
	}
	return payload.WorkflowRuns, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query map[string]string, v any) error {
	req, err := c.newRequest(ctx, path, query)
	if err != nil {
		return err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		msg := strings.TrimSpace(string(body))
		if res.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: %s", ErrNotFound, msg)
		}
		return fmt.Errorf("github api error (%d): %s", res.StatusCode, msg)
	}

	return json.NewDecoder(res.Body).Decode(v)
}

func (c *Client) newRequest(ctx context.Context, resource string, query map[string]string) (*http.Request, error) {
	u, err := url.Parse(c.baseURL + resource)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		q := u.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ghwatch")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return req, nil
}

func decorateRuns(payload []workflowRunPayload, cb func(*WorkflowRun)) []WorkflowRun {
	out := make([]WorkflowRun, 0, len(payload))
	for _, item := range payload {
		run := convertRun(item)
		if cb != nil {
			cb(&run)
		}
		out = append(out, run)
	}
	return out
}

func convertRun(payload workflowRunPayload) WorkflowRun {
	run := WorkflowRun{
		ID:            payload.ID,
		Name:          firstNonEmpty(payload.DisplayTitle, payload.Name),
		WorkflowName:  payload.Name,
		Status:        summarizeStatus(payload.Status, payload.Conclusion),
		StatusDetail:  buildStatusDetail(payload.Status, payload.Conclusion),
		HTMLURL:       payload.HTMLURL,
		HeadBranch:    payload.HeadBranch,
		HeadSHA:       payload.HeadSHA,
		Event:         payload.Event,
		LastUpdatedAt: payload.UpdatedAt,
	}
	if payload.Repository.FullName != "" {
		run.RepoFullName = payload.Repository.FullName
	}
	if run.Name == "" {
		run.Name = fmt.Sprintf("Run %d", payload.ID)
	}
	if pr := firstRunPullRequest(payload.PullRequests); pr != nil {
		run.Target = fmt.Sprintf("PR #%d", pr.Number)
		run.TargetURL = pr.HTMLURL
		run.PRNumber = pr.Number
		run.PRURL = pr.HTMLURL
	}
	if run.Target == "" {
		run.Target = cleanTarget(payload.HeadBranch, payload.Event)
	}
	if run.TargetURL == "" {
		run.TargetURL = payload.HTMLURL
	}
	return run
}

func firstRunPullRequest(prs []workflowRunPullRequest) *workflowRunPullRequest {
	if len(prs) == 0 {
		return nil
	}
	return &prs[0]
}

func cleanTarget(branch, event string) string {
	if branch != "" {
		return branch
	}
	if event != "" {
		return event
	}
	return "unknown"
}

func summarizeStatus(status, conclusion string) RunStatus {
	switch status {
	case "queued", "in_progress", "waiting", "requested":
		return RunStatusPending
	case "completed":
		switch conclusion {
		case "success":
			return RunStatusSuccess
		case "failure", "timed_out", "cancelled", "startup_failure", "stale":
			return RunStatusFailed
		default:
			return RunStatusPending
		}
	default:
		return RunStatusPending
	}
}

func buildStatusDetail(status, conclusion string) string {
	if status == "" {
		status = "unknown"
	}
	if conclusion == "" {
		return status
	}
	return fmt.Sprintf("%s/%s", status, conclusion)
}

type workflowRunsResponse struct {
	WorkflowRuns []workflowRunPayload `json:"workflow_runs"`
}

type workflowRunPayload struct {
	ID            int64                    `json:"id"`
	Name          string                   `json:"name"`
	DisplayTitle  string                   `json:"display_title"`
	Event         string                   `json:"event"`
	Status        string                   `json:"status"`
	Conclusion    string                   `json:"conclusion"`
	HTMLURL       string                   `json:"html_url"`
	HeadBranch    string                   `json:"head_branch"`
	HeadSHA       string                   `json:"head_sha"`
	UpdatedAt     time.Time                `json:"updated_at"`
	PullRequests  []workflowRunPullRequest `json:"pull_requests"`
	Repository    workflowRunRepository    `json:"repository"`
	RunStartedAt  time.Time                `json:"run_started_at"`
	HeadCommit    workflowRunHeadCommit    `json:"head_commit"`
	WorkflowID    int64                    `json:"workflow_id"`
	WorkflowName  string                   `json:"workflow_name"`
	OriginalTotal int                      `json:"run_attempt"`
	Links         workflowRunLinks         `json:"links"`
	Path          string                   `json:"path"`
	CreatedAt     time.Time                `json:"created_at"`
}

type workflowRunRepository struct {
	FullName string `json:"full_name"`
}

type workflowRunPullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Head    struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
}

type workflowRunHeadCommit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type workflowRunLinks struct {
	Workflow struct {
		Href string `json:"href"`
	} `json:"workflow"`
}

type pullRequestPayload struct {
	Number int `json:"number"`
	Head   struct {
		SHA string `json:"sha"`
		Ref string `json:"ref"`
	} `json:"head"`
	HTMLURL string `json:"html_url"`
}

// ErrNotFound can be returned when GitHub responds with 404.
var ErrNotFound = errors.New("resource not found")
