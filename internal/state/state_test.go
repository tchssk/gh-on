package state

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestState(t *testing.T) {
	t.Run("save and load", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "sub", "state.json")
		since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		s := &State{ETag: `W/"x"`, Since: since}
		s.Rule("a").Done["u@sha"] = since

		require.NoError(t, s.Save(path))
		got, err := Load(path)

		require.NoError(t, err)
		assert.Equal(t, `W/"x"`, got.ETag)
		assert.True(t, got.Since.Equal(since))
		assert.Contains(t, got.Rules["a"].Done, "u@sha")
	})

	t.Run("load missing", func(t *testing.T) {
		s, err := Load(filepath.Join(t.TempDir(), "none.json"))

		require.NoError(t, err)
		assert.Empty(t, s.ETag)
	})

	t.Run("retain", func(t *testing.T) {
		s := &State{}
		s.Rule("a")
		s.Rule("b")

		s.Retain([]string{"a"})

		assert.Contains(t, s.Rules, "a")
		assert.NotContains(t, s.Rules, "b")
	})
}
