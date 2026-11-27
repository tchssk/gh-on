package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Run("example", func(t *testing.T) {
		b, err := os.ReadFile("../../examples/rules.yaml")
		require.NoError(t, err)

		c, err := Parse(b)

		require.NoError(t, err)
		require.Len(t, c.Rules, 2)
		assert.Equal(t, KeyURLHead, c.Rules[0].Key)
		assert.Equal(t, KeyURL, c.Rules[1].Key)
	})

	t.Run("defaults", func(t *testing.T) {
		c, err := Parse([]byte(`
rules:
  - name: a
    repo: [o/r]
    query: is:pr repo:o/r
    command: echo "$@"
`))

		require.NoError(t, err)
		assert.Equal(t, KeyURL, c.Rules[0].Key)
		assert.Equal(t, []string{"o/r"}, c.Rules[0].Repo)
		assert.Zero(t, c.Rules[0].Timeout)
	})

	t.Run("timeout", func(t *testing.T) {
		c, err := Parse([]byte(`
rules:
  - {name: a, owner: [o], query: q, command: "true", timeout: 30m}
`))

		require.NoError(t, err)
		assert.Equal(t, 30*time.Minute, c.Rules[0].Timeout)
	})

	t.Run("negated repo with org", func(t *testing.T) {
		_, err := Parse([]byte(`
rules:
  - {name: a, owner: [o], query: "is:pr org:o -repo:o/old", command: "true"}
`))

		require.NoError(t, err)
	})

	for name, y := range map[string]string{
		"owner as a string": `
rules:
  - {name: a, owner: o, query: q, command: "true"}`,
		"no owner or repo": `
rules:
  - {name: a, query: q, command: "true"}`,
		"org with repo": `
rules:
  - {name: a, owner: [o], query: "org:o repo:o/r", command: "true"}`,
		"invalid key": `
rules:
  - {name: a, owner: [o], query: q, key: sha, command: "true"}`,
		"invalid repo": `
rules:
  - {name: a, repo: [r], query: q, command: "true"}`,
		"no command": `
rules:
  - {name: a, owner: [o], query: q}`,
		"duplicate name": `
rules:
  - {name: a, owner: [o], query: q, command: "true"}
  - {name: a, owner: [o], query: q, command: "true"}`,
		"unknown field": `
rules:
  - {name: a, owner: [o], query: q, command: "true", comand: x}`,
		"negative timeout": `
rules:
  - {name: a, owner: [o], query: q, command: "true", timeout: -1m}`,
		"invalid timeout": `
rules:
  - {name: a, owner: [o], query: q, command: "true", timeout: soon}`,
		"no rules": `rules: []`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(y))

			assert.Error(t, err)
		})
	}
}

func TestRule(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		r := &Rule{Type: []string{"PullRequest"}, Reason: []string{"review_requested"}, Owner: []string{"Org"}}

		assert.True(t, r.Match("PullRequest", "review_requested", "org/a"))
		assert.False(t, r.Match("Issue", "review_requested", "org/a"))
		assert.False(t, r.Match("PullRequest", "mention", "org/a"))
		assert.False(t, r.Match("PullRequest", "review_requested", "other/a"))
	})

	t.Run("match repo", func(t *testing.T) {
		r := &Rule{Repo: []string{"o/r"}}

		assert.True(t, r.Match("Issue", "assign", "o/r"))
		assert.False(t, r.Match("Issue", "assign", "o/other"))
	})
}
