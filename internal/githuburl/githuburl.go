package githuburl

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Kind identifies the type of GitHub URL provided by the user.
type Kind int

const (
	KindUnknown Kind = iota
	KindWorkflowRun
	KindPullRequest
	KindCommit
)

// Parsed represents a GitHub URL that the watcher understands.
type Parsed struct {
	Kind     Kind
	Owner    string
	Repo     string
	RunID    int64
	PRNumber int
	SHA      string
	RawURL   string
}

func (p Parsed) String() string {
	switch p.Kind {
	case KindWorkflowRun:
		return fmt.Sprintf("%s/%s run %d", p.Owner, p.Repo, p.RunID)
	case KindPullRequest:
		return fmt.Sprintf("%s/%s PR #%d", p.Owner, p.Repo, p.PRNumber)
	case KindCommit:
		return fmt.Sprintf("%s/%s commit %.7s", p.Owner, p.Repo, p.SHA)
	default:
		return "unknown"
	}
}

// Parse converts a user provided GitHub URL into a structured value that the
// application can work with.
func Parse(raw string) (Parsed, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Parsed{}, fmt.Errorf("empty URL")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Parsed{}, fmt.Errorf("invalid URL: %w", err)
	}

	if u.Host != "github.com" {
		return Parsed{}, fmt.Errorf("only github.com URLs are supported")
	}

	segments := splitPath(u.Path)
	if len(segments) < 2 {
		return Parsed{}, fmt.Errorf("URL must include owner and repo")
	}

	parsed := Parsed{
		Owner:  segments[0],
		Repo:   segments[1],
		RawURL: raw,
	}

	switch {
	case len(segments) >= 4 && segments[2] == "actions" && segments[3] == "runs":
		if len(segments) < 5 {
			return Parsed{}, fmt.Errorf("workflow run URL missing ID")
		}
		id, err := strconv.ParseInt(segments[4], 10, 64)
		if err != nil {
			return Parsed{}, fmt.Errorf("invalid run id: %w", err)
		}
		parsed.Kind = KindWorkflowRun
		parsed.RunID = id
	case len(segments) >= 4 && segments[2] == "pull":
		num, err := strconv.Atoi(segments[3])
		if err != nil {
			return Parsed{}, fmt.Errorf("invalid pull request number: %w", err)
		}
		parsed.Kind = KindPullRequest
		parsed.PRNumber = num
	case len(segments) >= 4 && segments[2] == "commit":
		if len(segments[3]) < 7 {
			return Parsed{}, fmt.Errorf("invalid commit SHA")
		}
		parsed.Kind = KindCommit
		parsed.SHA = segments[3]
	default:
		return Parsed{}, fmt.Errorf("unsupported GitHub URL path: %s", path.Join(segments...))
	}

	return parsed, nil
}

func splitPath(p string) []string {
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
