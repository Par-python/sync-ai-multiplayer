// Package context writes the plain-text project context consumed by any
// coding agent. It never contains source code or the participant token.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/syncroom/syncroom/internal/domain"
)

// Write atomically replaces all generated context files under root/.syncroom.
func Write(root, participantID string, snapshot domain.Snapshot) error {
	dir := filepath.Join(root, ".syncroom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create context directory: %w", err)
	}
	self := participantByID(snapshot.Participants, participantID)
	var context strings.Builder
	fmt.Fprintf(&context, "# Syncroom Project Context\n\nRoom: %s\nDefault branch: %s\n\n", snapshot.Room.Name, snapshot.Room.DefaultBranch)
	if self.ID != "" {
		fmt.Fprintf(&context, "## Your Assignment\n\nParticipant: %s + %s\n\n", self.Name, self.Agent)
	}
	context.WriteString("## Teammate Activity\n\n")
	for _, intent := range snapshot.Intents {
		if intent.ParticipantID == participantID {
			continue
		}
		name := participantByID(snapshot.Participants, intent.ParticipantID).Name
		fmt.Fprintf(&context, "- %s: %s", name, intent.Task)
		if len(intent.ExpectedPaths) > 0 {
			fmt.Fprintf(&context, " (%s)", strings.Join(intent.ExpectedPaths, ", "))
		}
		context.WriteString("\n")
	}
	if len(snapshot.Intents) == 0 {
		context.WriteString("- No published teammate activity yet.\n")
	}
	context.WriteString("\nRead decisions.md and updates.md before planning or publishing a checkpoint.\n")

	if err := writeAtomic(filepath.Join(dir, "context.md"), context.String()); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, "decisions.md"), "# Syncroom Shared Decisions\n\nNo shared decisions yet.\n"); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "updates.md"), "# Syncroom Updates\n\nNo routed updates yet.\n")
}

func participantByID(participants []domain.Participant, id string) domain.Participant {
	for _, participant := range participants {
		if participant.ID == id {
			return participant
		}
	}
	return domain.Participant{}
}

func writeAtomic(path, contents string) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".syncroom-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.WriteString(contents); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
