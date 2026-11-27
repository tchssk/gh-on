package cmd

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/tchssk/gh-on/internal/state"
)

type statusOptions struct {
	statePath string
}

// StatusCmd returns the "status" command.
func StatusCmd() *cobra.Command {
	opts := &statusOptions{statePath: state.Path()}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the last run",
		Long:  "Show the last run and the state of each rule.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd.OutOrStdout(), opts)
		},
	}

	return cmd
}

func runStatus(out io.Writer, opts *statusOptions) error {
	st, err := state.Load(opts.statePath)
	if err != nil {
		return fmt.Errorf("%s: %w", opts.statePath, err)
	}

	if st.LastRun.IsZero() {
		fmt.Fprintln(out, "Last run: never")
		return nil
	}
	fmt.Fprintf(out, "Last run: %s\n", st.LastRun.Local().Format(time.DateTime))
	if st.LastError != "" {
		fmt.Fprintf(out, "Last error: %s\n", st.LastError)
	}

	for _, n := range slices.Sorted(maps.Keys(st.Rules)) {
		r := st.Rules[n]
		line := fmt.Sprintf("%s: %d done", n, len(r.Done))
		if r.Pending {
			line += ", pending"
		}
		fmt.Fprintln(out, line)
	}
	return nil
}
