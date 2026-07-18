package context_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contextwriter "github.com/syncroom/syncroom/internal/context"
	"github.com/syncroom/syncroom/internal/domain"
)

func TestWriteRendersAtomically(t *testing.T) {
	root := t.TempDir()
	snapshot := domain.Snapshot{
		Room: domain.Room{Name: "Lost & Found", DefaultBranch: "main"},
		Participants: []domain.Participant{
			{ID: "alexi", Name: "Alexi", Agent: "Codex"},
			{ID: "abby", Name: "Abby", Agent: "Claude Code"},
		},
		Intents: []domain.Intent{{ParticipantID: "alexi", Task: "Build authentication", ExpectedPaths: []string{"packages/auth"}, UpdatedAt: time.Now()}},
	}
	if err := contextwriter.Write(root, "abby", snapshot); err != nil {
		t.Fatalf("write: %v", err)
	}
	contextBytes, err := os.ReadFile(filepath.Join(root, ".syncroom", "context.md"))
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	if text := string(contextBytes); !strings.Contains(text, "Abby") || !strings.Contains(text, "Build authentication") {
		t.Fatalf("context did not render assignment/team activity: %s", text)
	}
	for _, name := range []string{"decisions.md", "updates.md"} {
		if _, err := os.Stat(filepath.Join(root, ".syncroom", name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	matches, err := filepath.Glob(filepath.Join(root, ".syncroom", "*.tmp"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}
