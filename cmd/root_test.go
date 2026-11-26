package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmd(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)

		require.NoError(t, execute(root, []string{"--help"}))

		output := stdout.String() + stderr.String()
		assert.Contains(t, output, "Run a command when a GitHub notification matches a rule.")
	})

	t.Run("command path", func(t *testing.T) {
		root := RootCmd()

		assert.Equal(t, "gh on", root.CommandPath())
	})

	t.Run("version", func(t *testing.T) {
		root := RootCmd()
		var stdout bytes.Buffer
		root.SetOut(&stdout)

		require.NoError(t, execute(root, []string{"--version"}))

		assert.Equal(t, "gh on version dev\n", stdout.String())
	})

	t.Run("unknown flag", func(t *testing.T) {
		root := RootCmd()
		var stderr bytes.Buffer
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&stderr)

		err := execute(root, []string{"--no-such-flag"})

		require.Error(t, err)
		assert.Contains(t, stderr.String(), "unknown flag: --no-such-flag")
	})
}
