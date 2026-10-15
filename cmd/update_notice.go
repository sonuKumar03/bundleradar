package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/updater"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// noticeWait bounds how long a finished command waits for the update check.
const noticeWait = 300 * time.Millisecond

// quietCommands never print update notices (machine protocols and the updater itself).
var quietCommands = []string{"mcp", "update", "help", "completion", "__complete", "__completeNoDesc"}

// startUpdateCheck begins the daily update check in the background when it is
// appropriate: interactive stderr, no CI, not disabled, not a quiet command.
func startUpdateCheck(args []string, stderr io.Writer) <-chan string {
	if !shouldCheckForUpdates(args, stderr) {
		return nil
	}
	cacheFile, err := updater.DefaultCacheFile()
	if err != nil {
		return nil
	}
	n := &updater.Notifier{
		Client:    updater.New(bundleradar.ToolVersion),
		Current:   bundleradar.ToolVersion,
		CacheFile: cacheFile,
		Now:       time.Now,
	}
	ch := make(chan string, 1)
	go func() { ch <- n.Check(context.Background()) }()
	return ch
}

func printUpdateNotice(ch <-chan string, stderr io.Writer) {
	if ch == nil {
		return
	}
	select {
	case msg := <-ch:
		if msg != "" {
			fmt.Fprintf(stderr, "\n%s", msg)
		}
	case <-time.After(noticeWait):
	}
}

func shouldCheckForUpdates(args []string, stderr io.Writer) bool {
	if os.Getenv(updater.NoUpdateCheckEnv) != "" || os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return false
	}
	f, ok := stderr.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	for _, a := range args {
		if len(a) > 0 && a[0] != '-' {
			return !slices.Contains(quietCommands, a)
		}
	}
	return true
}
