package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// RootCmd returns the root "on" command.
func RootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "on <command>",
		Short: "Run commands on GitHub notifications",
		Long:  "Run a command when a GitHub notification matches a rule.",
		// Show "gh on" instead of just "on" in help and usage output.
		Annotations: map[string]string{
			cobra.CommandDisplayNameAnnotation: "gh on",
		},
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetVersionTemplate("gh on version {{.Version}}\n")

	root.AddCommand(RunCmd())

	return root
}

func execute(root *cobra.Command, args []string) error {
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	if err != nil {
		fmt.Fprintln(root.ErrOrStderr(), err)
	}
	return err
}

// Execute runs the root command with the process arguments and exits non-zero on failure.
func Execute() {
	if err := execute(RootCmd(), os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
