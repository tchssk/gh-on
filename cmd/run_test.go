package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tchssk/gh-on/internal/config"
	"github.com/tchssk/gh-on/internal/github"
	"github.com/tchssk/gh-on/internal/state"
)

type nopLauncher struct{ calls int }

func (l *nopLauncher) Launch(context.Context, *config.Rule, []string) error {
	l.calls++
	return nil
}

func newRunOptions(t *testing.T, client *github.MockClient) (*runOptions, *nopLauncher) {
	t.Helper()
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.yaml")
	require.NoError(t, os.WriteFile(rules, []byte(`
rules:
  - {name: a, owner: [o], query: q, command: "true"}
`), 0o644))
	l := &nopLauncher{}
	return &runOptions{
		configPath: rules,
		statePath:  filepath.Join(dir, "state.json"),
		lockPath:   filepath.Join(dir, "run.lock"),
		newClient:  func() (github.ClientOps, error) { return client, nil },
		launcher:   l,
	}, l
}

func TestRunCmd(t *testing.T) {
	ctx := context.Background()
	client := &github.MockClient{
		NotificationsFn: func(string) ([]github.Notification, string, bool, error) {
			var n github.Notification
			n.Repository.FullName = "o/r"
			n.UpdatedAt = time.Now()
			return []github.Notification{n}, `"e"`, false, nil
		},
		SearchIssuesFn: func(string) ([]github.Issue, error) {
			return []github.Issue{{HTMLURL: "https://github.com/o/r/issues/1", RepositoryURL: "https://api.github.com/repos/o/r"}}, nil
		},
	}

	t.Run("flags", func(t *testing.T) {
		cmd := RunCmd()

		assert.NotNil(t, cmd.Flags().Lookup("dry-run"))
		assert.NotNil(t, cmd.Flags().Lookup("all"))
	})

	t.Run("records the run", func(t *testing.T) {
		opts, l := newRunOptions(t, client)

		require.NoError(t, runRun(ctx, &bytes.Buffer{}, opts))

		assert.Equal(t, 1, l.calls)
		s, err := state.Load(opts.statePath)
		require.NoError(t, err)
		assert.False(t, s.LastRun.IsZero())
		assert.Empty(t, s.LastError)
	})

	t.Run("dry run", func(t *testing.T) {
		opts, l := newRunOptions(t, client)
		opts.dryRun = true
		var out bytes.Buffer

		require.NoError(t, runRun(ctx, &out, opts))

		assert.Zero(t, l.calls)
		assert.Contains(t, out.String(), "a: would run: true -- https://github.com/o/r/issues/1")
		assert.NoFileExists(t, opts.statePath)
	})

	t.Run("locked", func(t *testing.T) {
		opts, l := newRunOptions(t, client)
		held := flock.New(opts.lockPath)
		locked, err := held.TryLock()
		require.NoError(t, err)
		require.True(t, locked)
		defer held.Unlock()

		require.NoError(t, runRun(ctx, &bytes.Buffer{}, opts))

		assert.Zero(t, l.calls)
	})

	t.Run("missing rules", func(t *testing.T) {
		opts, _ := newRunOptions(t, client)
		opts.configPath = filepath.Join(t.TempDir(), "none.yaml")

		assert.Error(t, runRun(ctx, &bytes.Buffer{}, opts))
	})
}
