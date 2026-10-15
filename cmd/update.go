package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sonuKumar03/bundleradar/internal/updater"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func newUpdateCommand() *cobra.Command {
	var (
		check   bool
		version string
	)

	c := &cobra.Command{
		Use:   "update",
		Short: "Update bundleradar to the latest release",
		Long: `Download the latest bundleradar release for this platform, verify it against
the release checksums, and replace the running binary.

Use --check to only report whether a newer version exists, or --version to
install a specific release (including an older one).`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			out := c.OutOrStdout()
			current := bundleradar.ToolVersion
			client := updater.New(current)

			rel, err := client.Release(c.Context(), version)
			if err != nil {
				if version != "" {
					return &UsageError{Err: err}
				}
				return err
			}

			pinned := version != ""
			newer := updater.Newer(rel.Version(), current)
			if check {
				if newer {
					fmt.Fprintf(out, "bundleradar %s is installed; %s is available. Run `bundleradar update`.\n", current, rel.Version())
				} else {
					fmt.Fprintf(out, "bundleradar %s is up to date.\n", current)
				}
				return nil
			}
			if !pinned && !newer {
				fmt.Fprintf(out, "bundleradar %s is already up to date.\n", current)
				return nil
			}
			if pinned && rel.Version() == current {
				fmt.Fprintf(out, "bundleradar %s is already installed.\n", current)
				return nil
			}

			target, err := updater.ExecutablePath()
			if err != nil {
				return fmt.Errorf("locate running binary: %w", err)
			}
			goos, goarch := updater.CurrentPlatform()
			fmt.Fprintf(out, "Updating bundleradar %s → %s (%s/%s)...\n", current, rel.Version(), goos, goarch)
			if err := client.Install(c.Context(), rel, target, goos, goarch); err != nil {
				return err
			}
			fmt.Fprintf(out, "Updated %s to %s.\n", target, rel.Version())
			return nil
		},
	}
	c.Flags().BoolVar(&check, "check", false, "Only report whether a newer version is available")
	c.Flags().StringVar(&version, "version", "", "Install a specific release (e.g. v2.5.1)")
	return c
}
