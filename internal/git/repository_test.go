package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	gitrepo "github.com/syncroom/syncroom/internal/git"
)

func TestStateReportsBranchHeadAndDirtyPaths(t *testing.T) {
	root := t.TempDir()
	run(t, root, "init", "-b", "main")
	run(t, root, "config", "user.email", "test@example.test")
	run(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "add", "README.md")
	run(t, root, "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := gitrepo.State(root)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Branch != "main" || state.Head == "" || !state.Dirty || len(state.ChangedPaths) != 1 || state.ChangedPaths[0] != "README.md" {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func run(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
