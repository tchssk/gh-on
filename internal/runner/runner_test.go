package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tchssk/gh-on/internal/config"
	"github.com/tchssk/gh-on/internal/github"
	"github.com/tchssk/gh-on/internal/state"
)

const query = "is:pr is:open user-review-requested:@me org:o"

func at(hour int) time.Time { return time.Date(2026, 1, 1, hour, 0, 0, 0, time.UTC) }

func notification(typ, reason, repo string, updated time.Time) github.Notification {
	var n github.Notification
	n.Subject.Type = typ
	n.Reason = reason
	n.Repository.FullName = repo
	n.UpdatedAt = updated
	return n
}

func pull(repo string, number int) github.Issue {
	return github.Issue{
		HTMLURL:       fmt.Sprintf("https://github.com/%s/pull/%d", repo, number),
		RepositoryURL: "https://api.github.com/repos/" + repo,
		Number:        number,
		PullRequest:   &struct{}{},
	}
}

// fakeGitHub serves notifications with an ETag and returns 304 when it has not changed.
type fakeGitHub struct {
	etag     string
	ns       []github.Notification
	issues   []github.Issue
	heads    map[string]string
	searches int
	err      error
}

func (f *fakeGitHub) client() *github.MockClient {
	return &github.MockClient{
		NotificationsFn: func(etag string) ([]github.Notification, string, bool, error) {
			if etag != "" && etag == f.etag {
				return nil, etag, true, nil
			}
			return f.ns, f.etag, false, nil
		},
		SearchIssuesFn: func(q string) ([]github.Issue, error) {
			f.searches++
			if q != query {
				return nil, fmt.Errorf("unexpected query %q", q)
			}
			return f.issues, f.err
		},
		PullRequestHeadFn: func(repo string, number int) (string, error) {
			return f.heads[fmt.Sprintf("%s#%d", repo, number)], nil
		},
	}
}

// notify replaces the notifications with one updated at the given hour.
func (f *fakeGitHub) notify(hour int) {
	f.etag = fmt.Sprintf(`"e%d"`, hour)
	f.ns = []github.Notification{notification("PullRequest", "review_requested", "o/a", at(hour))}
}

type fakeLauncher struct {
	calls [][]string
	err   error
	check func()
}

func (l *fakeLauncher) Launch(_ context.Context, r *config.Rule, urls []string) error {
	if l.check != nil {
		l.check()
	}
	l.calls = append(l.calls, urls)
	return l.err
}

func newRunner(t *testing.T) (*Runner, *fakeGitHub, *fakeLauncher) {
	t.Helper()
	c, err := config.Parse([]byte(`
rules:
  - name: pr
    type: [PullRequest]
    reason: [review_requested]
    owner: [o]
    query: "` + query + `"
    key: url+head
    command: echo "$@"
`))
	require.NoError(t, err)
	gh := &fakeGitHub{issues: []github.Issue{pull("o/a", 1)}, heads: map[string]string{"o/a#1": "sha1"}}
	gh.notify(10)
	l := &fakeLauncher{}
	rn := &Runner{
		Config:    c,
		State:     &state.State{},
		StatePath: filepath.Join(t.TempDir(), "state.json"),
		Client:    gh.client(),
		Launcher:  l,
		Log:       &bytes.Buffer{},
		Now:       time.Now,
	}
	return rn, gh, l
}

func saved(t *testing.T, rn *Runner) *state.State {
	t.Helper()
	s, err := state.Load(rn.StatePath)
	require.NoError(t, err)
	return s
}

func TestRunner(t *testing.T) {
	ctx := context.Background()

	t.Run("runs and records", func(t *testing.T) {
		rn, _, l := newRunner(t)

		require.NoError(t, rn.Run(ctx))

		assert.Equal(t, [][]string{{"https://github.com/o/a/pull/1"}}, l.calls)
		s := saved(t, rn)
		assert.Equal(t, `"e10"`, s.ETag)
		assert.True(t, s.Since.Equal(at(10)))
		assert.False(t, s.Rules["pr"].Pending)
		assert.Contains(t, s.Rules["pr"].Done, "https://github.com/o/a/pull/1@sha1")
	})

	t.Run("not modified", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		require.NoError(t, rn.Run(ctx))
		gh.searches = 0

		require.NoError(t, rn.Run(ctx))

		assert.Zero(t, gh.searches)
		assert.Len(t, l.calls, 1)
	})

	t.Run("saves state before running", func(t *testing.T) {
		rn, _, l := newRunner(t)
		l.check = func() {
			s := saved(t, rn)
			assert.Equal(t, `"e10"`, s.ETag)
			assert.False(t, s.Rules["pr"].Pending)
		}

		require.NoError(t, rn.Run(ctx))
	})

	t.Run("same head is not run again", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		require.NoError(t, rn.Run(ctx))
		gh.notify(11) // e.g. a comment

		require.NoError(t, rn.Run(ctx))

		assert.Len(t, l.calls, 1)
	})

	t.Run("new head is run again", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		require.NoError(t, rn.Run(ctx))
		gh.notify(11)
		gh.heads["o/a#1"] = "sha2"

		require.NoError(t, rn.Run(ctx))

		assert.Len(t, l.calls, 2)
		assert.Equal(t, []string{"https://github.com/o/a/pull/1@sha2"}, keys(rn.State.Rules["pr"].Done))
	})

	t.Run("old and unmatched notifications", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		rn.State.Since = at(10)
		gh.ns = []github.Notification{
			notification("PullRequest", "review_requested", "o/a", at(10)), // not newer than Since
			notification("PullRequest", "mention", "o/a", at(11)),
			notification("PullRequest", "review_requested", "x/a", at(11)),
		}

		require.NoError(t, rn.Run(ctx))

		assert.Zero(t, gh.searches)
		assert.Empty(t, l.calls)
		assert.True(t, rn.State.Since.Equal(at(11)))
	})

	t.Run("search results outside the rule", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		gh.issues = append(gh.issues, pull("x/b", 2))

		require.NoError(t, rn.Run(ctx))

		assert.Equal(t, [][]string{{"https://github.com/o/a/pull/1"}}, l.calls)
	})

	t.Run("no targets", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		gh.issues = nil // e.g. a team review request

		require.NoError(t, rn.Run(ctx))

		assert.Empty(t, l.calls)
		assert.False(t, saved(t, rn).Rules["pr"].Pending)
	})

	t.Run("failed command", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		l.err = errors.New("exit status 1")

		require.Error(t, rn.Run(ctx))
		s := saved(t, rn)
		assert.Empty(t, s.Rules["pr"].Done)
		assert.False(t, s.Rules["pr"].Pending)

		// Picked up again on the next notification.
		l.err = nil
		gh.notify(11)
		require.NoError(t, rn.Run(ctx))
		assert.Len(t, l.calls, 2)
	})

	t.Run("skip if exists", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		busy := filepath.Join(t.TempDir(), ".lock")
		require.NoError(t, os.Mkdir(busy, 0o755))
		rn.Config.Rules[0].SkipIfExists = busy

		require.NoError(t, rn.Run(ctx))
		assert.Empty(t, l.calls)
		assert.Zero(t, gh.searches)
		assert.True(t, saved(t, rn).Rules["pr"].Pending)

		// Checked again even when notifications are not modified.
		require.NoError(t, os.Remove(busy))
		require.NoError(t, rn.Run(ctx))
		assert.Len(t, l.calls, 1)
	})

	t.Run("search error", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		gh.err = errors.New("502")

		require.Error(t, rn.Run(ctx))
		assert.True(t, saved(t, rn).Rules["pr"].Pending)

		gh.err = nil
		require.NoError(t, rn.Run(ctx))
		assert.Len(t, l.calls, 1)
	})

	t.Run("forgets closed targets", func(t *testing.T) {
		rn, gh, l := newRunner(t)
		require.NoError(t, rn.Run(ctx))
		rn.All = true
		gh.issues = nil

		require.NoError(t, rn.Run(ctx))
		assert.Empty(t, rn.State.Rules["pr"].Done)

		// Requested again.
		gh.issues = []github.Issue{pull("o/a", 1)}
		require.NoError(t, rn.Run(ctx))
		assert.Len(t, l.calls, 2)
	})

	t.Run("dry run", func(t *testing.T) {
		rn, _, l := newRunner(t)
		rn.DryRun = true

		require.NoError(t, rn.Run(ctx))

		assert.Empty(t, l.calls)
		assert.NoFileExists(t, rn.StatePath)
		assert.Contains(t, rn.Log.(*bytes.Buffer).String(), `pr: would run: echo "$@" -- https://github.com/o/a/pull/1`)
	})

	t.Run("drops removed rules", func(t *testing.T) {
		rn, _, _ := newRunner(t)
		rn.State.Rule("old").Pending = true

		require.NoError(t, rn.Run(ctx))

		assert.NotContains(t, saved(t, rn).Rules, "old")
	})
}

func TestShell(t *testing.T) {
	t.Run("passes urls as positional parameters", func(t *testing.T) {
		var out bytes.Buffer
		s := &Shell{Stdout: &out, Stderr: &out}
		r := &config.Rule{Command: `printf '%s|' "$#" "$@"`, Dir: t.TempDir()}

		require.NoError(t, s.Launch(context.Background(), r, []string{"u 1", "u2"}))

		assert.Equal(t, "2|u 1|u2|", out.String())
	})

	t.Run("exit status", func(t *testing.T) {
		s := &Shell{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
		r := &config.Rule{Command: "exit 3", Dir: t.TempDir()}

		assert.Error(t, s.Launch(context.Background(), r, nil))
	})

	t.Run("timeout stops the command and its children", func(t *testing.T) {
		s := &Shell{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
		// sleep runs as a child of sh and keeps the output pipe open.
		r := &config.Rule{Command: "sleep 30; echo done", Dir: t.TempDir(), Timeout: 100 * time.Millisecond}
		start := time.Now()

		err := s.Launch(context.Background(), r, nil)

		assert.EqualError(t, err, "timed out after 100ms")
		assert.Less(t, time.Since(start), 5*time.Second)
	})
}

func keys(m map[string]time.Time) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
