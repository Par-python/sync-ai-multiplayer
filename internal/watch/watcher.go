// Package watch maintains local generated context from coordinator snapshots.
package watch

import (
	"context"
	"time"

	contextwriter "github.com/syncroom/syncroom/internal/context"
	"github.com/syncroom/syncroom/internal/domain"
	gitrepo "github.com/syncroom/syncroom/internal/git"
)

// SnapshotClient is the observer's minimal coordinator dependency.
type SnapshotClient interface {
	Snapshot(context.Context, string) (domain.Snapshot, error)
}
type ActivityClient interface {
	PublishActivity(context.Context, string, string, string, bool, []string) error
}

// Sync fetches one snapshot and atomically refreshes local context files.
func Sync(ctx context.Context, root, roomID, participantID string, client SnapshotClient) error {
	if publisher, ok := client.(ActivityClient); ok {
		if state, err := gitrepo.State(root); err == nil {
			_ = publisher.PublishActivity(ctx, roomID, state.Branch, state.Head, state.Dirty, state.ChangedPaths)
		}
	}
	snapshot, err := client.Snapshot(ctx, roomID)
	if err != nil {
		return err
	}
	return contextwriter.Write(root, participantID, snapshot)
}

// Run continuously refreshes generated context until cancelled.
func Run(ctx context.Context, root, roomID, participantID string, client SnapshotClient, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if err := Sync(ctx, root, roomID, participantID, client); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := Sync(ctx, root, roomID, participantID, client); err != nil {
				return err
			}
		}
	}
}
