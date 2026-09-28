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
		mcpCommand(),
	)
	return root
}

func Execute(args []string, stdout, stderr io.Writer) int {
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
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
