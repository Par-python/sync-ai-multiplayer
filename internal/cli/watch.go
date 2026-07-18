package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/syncroom/syncroom/internal/client"
	gitrepo "github.com/syncroom/syncroom/internal/git"
	"github.com/syncroom/syncroom/internal/local"
	"github.com/syncroom/syncroom/internal/watch"
)

// Watch refreshes local generated context until the caller cancels it.
func Watch(ctx context.Context, root string, interval time.Duration) error {
	root, err := gitrepo.Root(root)
	if err != nil {
		return fmt.Errorf("locate Git repository: %w", err)
	}
	config, err := local.Read(root)
	if err != nil {
		return err
	}
	return watch.Run(ctx, root, config.RoomID, config.ParticipantID, &client.API{BaseURL: config.ServerURL, Token: config.Token}, interval)
}
