// Package cli implements the repository-local Syncroom commands.
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/syncroom/syncroom/internal/client"
	contextwriter "github.com/syncroom/syncroom/internal/context"
	gitrepo "github.com/syncroom/syncroom/internal/git"
	"github.com/syncroom/syncroom/internal/local"
)

// AttachInput contains the explicitly supplied enrollment details.
type AttachInput struct {
	Root      string
	ServerURL string
	JoinCode  string
	Name      string
	Agent     string
	Branch    string
}

// Attach enrolls an existing clone and generates its first local context.
func Attach(ctx context.Context, input AttachInput) error {
	if strings.TrimSpace(input.ServerURL) == "" || strings.TrimSpace(input.JoinCode) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Agent) == "" {
		return fmt.Errorf("server URL, join code, name, and agent are required")
	}
	root, err := gitrepo.Root(input.Root)
	if err != nil {
		return fmt.Errorf("locate Git repository: %w", err)
	}
	if !gitrepo.HasOrigin(root) {
		return fmt.Errorf("repository must have an origin remote")
	}
	enrollment, err := client.Join(ctx, input.ServerURL, input.JoinCode, input.Name, input.Agent)
	if err != nil {
		return err
	}
	api := &client.API{BaseURL: input.ServerURL, Token: enrollment.Token}
	snapshot, err := api.Snapshot(ctx, enrollment.Participant.RoomID)
	if err != nil {
		return err
	}
	state, err := gitrepo.State(root)
	if err != nil {
		return err
	}
	if state.Branch == snapshot.Room.DefaultBranch {
		if strings.TrimSpace(input.Branch) == "" {
			return fmt.Errorf("current branch is protected default %q; provide --branch", state.Branch)
		}
		if err := gitrepo.SwitchNew(root, input.Branch); err != nil {
			return err
		}
	}
	config := local.Config{ServerURL: input.ServerURL, RoomID: enrollment.Participant.RoomID, ParticipantID: enrollment.Participant.ID, Token: enrollment.Token}
	if err := local.Write(root, config); err != nil {
		return err
	}
	return contextwriter.Write(root, config.ParticipantID, snapshot)
}
