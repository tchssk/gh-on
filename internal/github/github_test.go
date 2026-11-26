package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIssue(t *testing.T) {
	t.Run("repo", func(t *testing.T) {
		i := Issue{RepositoryURL: "https://api.github.com/repos/o/r"}

		assert.Equal(t, "o/r", i.Repo())
	})
}
