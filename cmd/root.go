package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// Numeric exit codes conforming to automation contract
const (
	ExitCodeSuccess         = 0 // Command succeeded, analysis passed, policy satisfied
	ExitCodePolicyViolation = 1 // Explicit budget breached or disallowed package detected
	ExitCodeUsage           = 2 // Invalid flags, syntax, config file, or argument error
	ExitCodeExecution       = 3 // Runtime failure, missing build artifacts, I/O error
)

// PolicyViolationError is a CLI-level alias of the core typed error.
type PolicyViolationError = core.PolicyViolationError

// UsageError is a CLI-level alias of the core typed error.
type UsageError = core.UsageError

// MapErrorToExitCode maps an error to its documented numeric exit code.
// Classification is type-based: UsageError (or Cobra flag errors) -> 2,
// PolicyViolationError -> 1, everything else -> 3.
func MapErrorToExitCode(err error) int {
	if err == nil {
		return ExitCodeSuccess
	}
	var pErr *core.PolicyViolationError
	if errors.As(err, &pErr) {
		return ExitCodePolicyViolation
	}
	var uErr *core.UsageError
	if errors.As(err, &uErr) {
		return ExitCodeUsage
	}
	return ExitCodeExecution
}

// NewRootCommand creates and configures the root bundleradar cobra.Command.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "bundleradar",
		Short:   "Universal bundle analyzer, diff engine, and size budget gate",
		Version: bundleradar.ToolVersion,
		Long: `bundleradar is a fast, universal CLI and AI agent skill for JavaScript and web bundle analysis.
It calculates accurate initial vs. async byte totals, attributes npm package sizes,
estimates Gzip wire transfer sizes, tracks regressions, and enforces bundle size budgets in CI.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Cobra's default root arg check returns a plain error that maps to
		// the execution-failure exit code; an unknown subcommand is a usage error.
		Args: func(c *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &UsageError{Err: fmt.Errorf("unknown command %q for %q", args[0], c.CommandPath())}
			}
			return nil
		},
		RunE: func(c *cobra.Command, args []string) error { return c.Help() },
	}
	// Classify Cobra flag/argument errors as typed usage errors so exit-code
	// mapping never depends on error message strings.
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return &UsageError{Err: err}
	})
	root.AddCommand(
		newScanCommand(),
		newDiffCommand(),
		newGateCommand(),
		newWorkspaceCommand(),
		newUICommand(),
		newHistoryCommand(),
		newUpdateCommand(),
		mcpCommand(),
	)
	root.InitDefaultCompletionCmd()
	if comp, _, err := root.Find([]string{"completion"}); err == nil && comp != nil && comp.Name() == "completion" {
		comp.Args = func(c *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &UsageError{Err: fmt.Errorf("unknown shell %q; supported: bash, zsh, fish, powershell", args[0])}
			}
			return nil
		}
		comp.RunE = func(c *cobra.Command, args []string) error { return c.Help() }
	}
	hardenArgs(root)
	return root
}

// hardenArgs makes argument-count errors and unknown subcommands of group
// commands (history, workspace, ...) usage errors, so they exit 2 like flag errors.
func hardenArgs(c *cobra.Command) {
	for _, sub := range c.Commands() {
		hardenArgs(sub)
	}
	if c.Args != nil {
		orig := c.Args
		c.Args = func(cc *cobra.Command, args []string) error {
			err := orig(cc, args)
			var u *UsageError
			if err == nil || errors.As(err, &u) {
				return err
			}
			return &UsageError{Err: err}
		}
		return
	}
	if c.HasSubCommands() && !c.Runnable() {
		c.Args = func(cc *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &UsageError{Err: fmt.Errorf("unknown command %q for %q", args[0], cc.CommandPath())}
			}
			return nil
		}
		c.RunE = func(cc *cobra.Command, args []string) error { return cc.Help() }
	}
}

func Execute(args []string, stdout, stderr io.Writer) int {
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	notice := startUpdateCheck(args, stderr)
	err := root.Execute()
	defer printUpdateNotice(notice, stderr)

	if err != nil {
		fmt.Fprintf(stderr, "bundleradar: %v\n", err)
		return MapErrorToExitCode(err)
	}
	return ExitCodeSuccess
}

func getOutputWriter(c *cobra.Command, output string) (io.Writer, func() error, error) {
	if output == "" {
		return c.OutOrStdout(), func() error { return nil }, nil
	}
	dir := filepath.Dir(output)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, fmt.Errorf("create directory %q: %w", dir, err)
		}
	}
	f, err := os.Create(output)
	if err != nil {
		return nil, nil, fmt.Errorf("create output file %q: %w", output, err)
	}
	return f, f.Close, nil
}
