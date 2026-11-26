package runner

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/tchssk/gh-on/internal/config"
)

// Shell runs a rule's command with sh -c, passing the URLs as positional parameters.
type Shell struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (s *Shell) Launch(ctx context.Context, r *config.Rule, urls []string) error {
	args := append([]string{"-c", r.Command, "gh-on"}, urls...)
	cmd := exec.CommandContext(ctx, "/bin/sh", args...)
	cmd.Stdout = s.Stdout
	cmd.Stderr = s.Stderr
	cmd.Dir = r.Dir
	if cmd.Dir == "" {
		cmd.Dir, _ = os.UserHomeDir()
	}
	return cmd.Run()
}
