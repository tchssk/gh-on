package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/spf13/cobra"
	"github.com/tchssk/gh-on/internal/config"
	"github.com/tchssk/gh-on/internal/github"
	"github.com/tchssk/gh-on/internal/runner"
	"github.com/tchssk/gh-on/internal/state"
)

type runOptions struct {
	dryRun bool
	all    bool

	configPath string
	statePath  string
	lockPath   string
	newClient  func() (github.ClientOps, error)
	launcher   runner.Launcher
}

// RunCmd returns the "run" command.
func RunCmd() *cobra.Command {
	opts := &runOptions{
		configPath: config.Path(),
		statePath:  state.Path(),
		lockPath:   filepath.Join(state.Dir(), "run.lock"),
		newClient:  func() (github.ClientOps, error) { return github.NewClient() },
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Check notifications once",
		Long:  "Check notifications once and run the commands of matching rules.",
		Example: `  # Show what would run
  $ gh on run --dry-run --all`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.launcher == nil {
				opts.launcher = &runner.Shell{Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()}
			}
			return runRun(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print the commands instead of running them")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Check every rule without looking at notifications")

	return cmd
}

func runRun(ctx context.Context, out io.Writer, opts *runOptions) error {
	if err := os.MkdirAll(filepath.Dir(opts.lockPath), 0o700); err != nil {
		return err
	}
	fl := flock.New(opts.lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return err
	}
	if !locked {
		// launchd does not overlap runs but cron does; just skip this one.
		if opts.dryRun {
			fmt.Fprintln(out, "another run is in progress")
		}
		return nil
	}
	defer fl.Unlock()

	c, err := config.Load(opts.configPath)
	if err != nil {
		return fmt.Errorf("%s: %w", opts.configPath, err)
	}
	st, err := state.Load(opts.statePath)
	if err != nil {
		return fmt.Errorf("%s: %w", opts.statePath, err)
	}
	client, err := opts.newClient()
	if err != nil {
		return err
	}

	rn := &runner.Runner{
		Config:    c,
		State:     st,
		StatePath: opts.statePath,
		Client:    client,
		Launcher:  opts.launcher,
		Log:       out,
		Now:       time.Now,
		DryRun:    opts.dryRun,
		All:       opts.all,
	}
	runErr := rn.Run(ctx)
	if opts.dryRun {
		return runErr
	}
	st.LastRun = time.Now()
	st.LastError = ""
	if runErr != nil {
		st.LastError = runErr.Error()
	}
	return errors.Join(runErr, st.Save(opts.statePath))
}
