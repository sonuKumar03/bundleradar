package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// NoUpdateCheckEnv disables the background update check when non-empty.
const NoUpdateCheckEnv = "BUNDLERADAR_NO_UPDATE_CHECK"

const (
	checkInterval  = 24 * time.Hour
	noticeInterval = 7 * 24 * time.Hour
	retryInterval  = time.Hour
	checkTimeout   = 2 * time.Second
)

type noticeState struct {
	CheckedAt  time.Time `json:"checkedAt"`
	Latest     string    `json:"latest"`
	NotifiedAt time.Time `json:"notifiedAt"`
}

// Notifier decides whether to tell the user a newer release exists. It hits
// the network at most once a day and prints at most one notice every 7 days.
type Notifier struct {
	Client    *Client
	Current   string
	CacheFile string
	Now       func() time.Time
}

// DefaultCacheFile is the per-user cache location for update-check state.
func DefaultCacheFile() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bundleradar", "update-check.json"), nil
}

// Check returns the notice text to show, or "" when there is nothing to say.
// Network and cache failures are swallowed: an update check must never break a command.
func (n *Notifier) Check(ctx context.Context) string {
	now := n.Now()
	state := n.load()

	if now.Sub(state.CheckedAt) >= checkInterval {
		cctx, cancel := context.WithTimeout(ctx, checkTimeout)
		rel, err := n.Client.Release(cctx, "")
		cancel()
		if err == nil {
			state.Latest = rel.Version()
			state.CheckedAt = now
		} else {
			// Retry in an hour instead of on every command while offline.
			state.CheckedAt = now.Add(retryInterval - checkInterval)
		}
		n.save(state)
	}

	if !Newer(state.Latest, n.Current) || now.Sub(state.NotifiedAt) < noticeInterval {
		return ""
	}
	state.NotifiedAt = now
	n.save(state)
	return fmt.Sprintf("A new version of bundleradar is available: %s → %s\nRun `bundleradar update` to upgrade (silence with %s=1).\n",
		n.Current, state.Latest, NoUpdateCheckEnv)
}

func (n *Notifier) load() noticeState {
	var s noticeState
	if data, err := os.ReadFile(n.CacheFile); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func (n *Notifier) save(s noticeState) {
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(n.CacheFile), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(n.CacheFile, data, 0o644)
}
