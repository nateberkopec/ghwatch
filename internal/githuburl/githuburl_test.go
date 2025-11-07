package githuburl

import (
	"testing"
)

func TestParseWorkflowRun(t *testing.T) {
	url := "https://github.com/owner/repo/actions/runs/123456"
	parsed, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.Kind != KindWorkflowRun {
		t.Fatalf("expected KindWorkflowRun, got %v", parsed.Kind)
	}
	if parsed.Owner != "owner" || parsed.Repo != "repo" || parsed.RunID != 123456 {
		t.Fatalf("unexpected parsed fields: %#v", parsed)
	}
}

func TestParsePullRequest(t *testing.T) {
	url := "https://github.com/owner/repo/pull/42"
	parsed, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.Kind != KindPullRequest || parsed.PRNumber != 42 {
		t.Fatalf("unexpected parsed result: %#v", parsed)
	}
}

func TestParseCommit(t *testing.T) {
	url := "https://github.com/owner/repo/commit/0123456789abcdef"
	parsed, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.Kind != KindCommit || parsed.SHA != "0123456789abcdef" {
		t.Fatalf("unexpected parsed result: %#v", parsed)
	}
}

func TestParseInvalidHost(t *testing.T) {
	_, err := Parse("https://example.com/owner/repo")
	if err == nil {
		t.Fatal("expected error for unsupported host")
	}
}

func TestParseUnsupportedPath(t *testing.T) {
	_, err := Parse("https://github.com/owner/repo/releases")
	if err == nil {
		t.Fatal("expected error for unsupported path")
	}
}
