package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func newHistoryCommand() *cobra.Command {
	var historyFile string

	parent := &cobra.Command{
		Use:   "history",
		Short: "Manage persistent build history checkpoints",
	}

	saveCmd := &cobra.Command{
		Use:   "save [stats.json]",
		Short: "Record a build checkpoint to the history file",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			label, _ := c.Flags().GetString("label")
			statsPath := args[0]

			client := bundleradar.New()
			bundle, err := client.Scan(c.Context(), bundleradar.ScanOptions{
				StatsPath: statsPath,
			})
			if err != nil {
				return fmt.Errorf("scan bundle: %w", err)
			}

			checkpoints, baselineID, err := server.LoadHistoryFile(historyFile)
			if err != nil {
				return err
			}

			cp := buildCheckpointFromBundle(bundle, label)
			if len(checkpoints) == 0 {
				cp.IsBaseline = true
				baselineID = cp.ID
			}
			checkpoints = append(checkpoints, cp)

			if err := server.SaveHistoryFile(historyFile, checkpoints, baselineID); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "Checkpoint %s saved (%s)\n", cp.ID, cp.Label)
			return nil
		},
	}
	saveCmd.Flags().String("label", "", "Human-readable label for this checkpoint")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all build checkpoints in the history file",
		RunE: func(c *cobra.Command, args []string) error {
			checkpoints, baselineID, err := server.LoadHistoryFile(historyFile)
			if err != nil {
				return err
			}
			if len(checkpoints) == 0 {
				fmt.Fprintln(c.OutOrStdout(), "No checkpoints found.")
				return nil
			}

			w := c.OutOrStdout()
			fmt.Fprintf(w, "%-22s  %-24s  %-12s  %-12s  %s\n", "ID", "TIMESTAMP", "INITIAL", "TOTAL", "LABEL")
			fmt.Fprintf(w, "%-22s  %-24s  %-12s  %-12s  %s\n", "──────────────────────", "────────────────────────", "────────────", "────────────", "─────")
			for _, cp := range checkpoints {
				marker := ""
				if cp.ID == baselineID {
					marker = " *baseline"
				}
				fmt.Fprintf(w, "%-22s  %-24s  %-12s  %-12s  %s%s\n",
					cp.ID,
					cp.Timestamp.Format("2006-01-02 15:04:05"),
					bundleradar.FormatBytes(cp.InitialBytes),
					bundleradar.FormatBytes(cp.TotalBytes),
					cp.Label,
					marker,
				)
			}
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show details for a specific checkpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			checkpoints, baselineID, err := server.LoadHistoryFile(historyFile)
			if err != nil {
				return err
			}

			var cp *server.BuildCheckpoint
			for _, candidate := range checkpoints {
				if candidate.ID == id {
					cp = candidate
					break
				}
			}
			if cp == nil {
				return &UsageError{Err: fmt.Errorf("checkpoint %q not found", id)}
			}

			w := c.OutOrStdout()
			fmt.Fprintf(w, "Checkpoint: %s\n", cp.ID)
			fmt.Fprintf(w, "Timestamp:  %s\n", cp.Timestamp.Format(time.RFC3339))
			fmt.Fprintf(w, "Label:      %s\n", cp.Label)
			fmt.Fprintf(w, "Baseline:   %v\n", cp.ID == baselineID)
			fmt.Fprintf(w, "Initial JS: %s\n", bundleradar.FormatBytes(cp.InitialBytes))
			fmt.Fprintf(w, "Async JS:   %s\n", bundleradar.FormatBytes(cp.AsyncBytes))
			fmt.Fprintf(w, "Total JS:   %s\n", bundleradar.FormatBytes(cp.TotalBytes))
			fmt.Fprintf(w, "Chunks:     %d\n", cp.ChunksCount)
			fmt.Fprintf(w, "Packages:   %d\n", cp.PkgsCount)
			return nil
		},
	}

	parent.PersistentFlags().StringVar(&historyFile, "history-file", ".bundleradar-history.json", "Path to history file")
	parent.AddCommand(saveCmd, listCmd, showCmd)
	return parent
}

func buildCheckpointFromBundle(bundle *core.Bundle, label string) *server.BuildCheckpoint {
	cpID := fmt.Sprintf("build-%d", time.Now().UnixMilli())
	if label == "" {
		label = cpID
	}

	var initBytes, initGzip, asyncBytes, totalBytes int64
	for _, ep := range bundle.Entrypoints {
		initBytes += ep.InitialBytes
		initGzip += ep.InitialGzipBytes
		asyncBytes += ep.AsyncBytes
	}
	for _, ch := range bundle.Chunks {
		totalBytes += ch.SizeBytes
	}

	pkgCount := len(bundle.TopPackages(100))

	return &server.BuildCheckpoint{
		ID:               cpID,
		Timestamp:        time.Now(),
		Label:            label,
		InitialBytes:     initBytes,
		InitialGzipBytes: initGzip,
		AsyncBytes:       asyncBytes,
		TotalBytes:       totalBytes,
		ChunksCount:      len(bundle.Chunks),
		PkgsCount:        pkgCount,
	}
}
