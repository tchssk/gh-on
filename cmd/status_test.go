package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tchssk/gh-on/internal/state"
)

func TestStatusCmd(t *testing.T) {
	t.Run("never run", func(t *testing.T) {
		opts := &statusOptions{statePath: filepath.Join(t.TempDir(), "state.json")}
		var out bytes.Buffer

		require.NoError(t, runStatus(&out, opts))

		assert.Equal(t, "Last run: never\n", out.String())
	})

	t.Run("last run", func(t *testing.T) {
		opts := &statusOptions{statePath: filepath.Join(t.TempDir(), "state.json")}
		st := &state.State{
			LastRun:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local),
			LastError: "exit status 1",
		}
		st.Rule("b").Pending = true
		st.Rule("a").Done["https://github.com/o/r/pull/1"] = time.Now()
		require.NoError(t, st.Save(opts.statePath))
		var out bytes.Buffer

		require.NoError(t, runStatus(&out, opts))

		assert.Equal(t, `Last run: 2026-01-02 03:04:05
Last error: exit status 1
a: 1 done
b: 0 done, pending
`, out.String())
	})

	t.Run("broken state", func(t *testing.T) {
		opts := &statusOptions{statePath: filepath.Join(t.TempDir(), "state.json")}
		require.NoError(t, os.WriteFile(opts.statePath, []byte("{"), 0o600))

		assert.Error(t, runStatus(&bytes.Buffer{}, opts))
	})
}
