package cli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/syncroom/syncroom/internal/cli"
	"github.com/syncroom/syncroom/internal/server"
	"github.com/syncroom/syncroom/internal/store"
	"net/http/httptest"
)

func TestAttachWritesLocalConfigurationAndContext(t *testing.T) {
	root := t.TempDir()
	run(t, root, "init", "-b", "feature/abby")
	run(t, root, "config", "user.email", "abby@example.test")
	run(t, root, "config", "user.name", "Abby")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("syncroom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "add", "README.md")
	run(t, root, "commit", "-m", "initial")
	remote := filepath.Join(t.TempDir(), "remote.git")
	command := exec.Command("git", "init", "--bare", remote)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("init remote: %v %s", err, output)
	}
	run(t, root, "remote", "add", "origin", remote)

	db, err := store.Open(filepath.Join(t.TempDir(), "syncroom.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	room, err := db.CreateRoom(context.Background(), store.CreateRoomInput{Name: "Room", Repo: remote, DefaultBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(server.New(db))
	t.Cleanup(api.Close)

	if err := cli.Attach(context.Background(), cli.AttachInput{Root: root, ServerURL: api.URL, JoinCode: room.JoinCode, Name: "Abby", Agent: "Claude Code"}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".syncroom", "config.json")); err != nil {
		t.Fatalf("config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".syncroom", "context.md")); err != nil {
		t.Fatalf("context: %v", err)
	}
}

func run(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
