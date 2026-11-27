package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/tchssk/gh-on/internal/config"
)

// killDelay is how long a timed-out command has to exit after SIGTERM before it is killed.
const killDelay = 10 * time.Second

// Shell runs a rule's command with sh -c, passing the URLs as positional parameters.
type Shell struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (s *Shell) Launch(ctx context.Context, r *config.Rule, urls []string) error {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	args := append([]string{"-c", r.Command, "gh-on"}, urls...)
	cmd := exec.CommandContext(ctx, "/bin/sh", args...)
	cmd.Stdout = s.Stdout
	cmd.Stderr = s.Stderr
	cmd.Dir = r.Dir
	if cmd.Dir == "" {
		cmd.Dir, _ = os.UserHomeDir()
	}
	// Run the command in its own process group so that stopping it also stops its children.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = killDelay

	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timed out after %s", r.Timeout)
	}
	return err
}
