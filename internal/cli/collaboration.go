package cli

import (
	"context"
	"strings"

	"github.com/syncroom/syncroom/internal/client"
	"github.com/syncroom/syncroom/internal/domain"
	gitrepo "github.com/syncroom/syncroom/internal/git"
	"github.com/syncroom/syncroom/internal/local"
)

// PublishIntent sends a repository-local participant's declared scope.
func PublishIntent(ctx context.Context, root, task, objective, areas string, status domain.IntentStatus) error {
	root, err := gitrepo.Root(root)
	if err != nil {
		return err
	}
	config, err := local.Read(root)
	if err != nil {
		return err
	}
	paths := splitAreas(areas)
	_, err = (&client.API{BaseURL: config.ServerURL, Token: config.Token}).PublishIntent(ctx, config.RoomID, task, objective, paths, status)
	return err
}

// PublishDecision sends a repository-local participant's shared decision.
func PublishDecision(ctx context.Context, root, title, body string) error {
	root, err := gitrepo.Root(root)
	if err != nil {
		return err
	}
	config, err := local.Read(root)
	if err != nil {
		return err
	}
	_, err = (&client.API{BaseURL: config.ServerURL, Token: config.Token}).PublishDecision(ctx, config.RoomID, title, body)
	return err
}

func splitAreas(value string) []string {
	var paths []string
	for _, area := range strings.Split(value, ",") {
		if area = strings.TrimSpace(area); area != "" {
			paths = append(paths, area)
		}
	}
	return paths
}
