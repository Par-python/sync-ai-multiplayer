package client_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/syncroom/syncroom/internal/client"
	"github.com/syncroom/syncroom/internal/server"
	"github.com/syncroom/syncroom/internal/store"
)

func TestJoinAndSnapshot(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "syncroom.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	room, err := db.CreateRoom(context.Background(), store.CreateRoomInput{Name: "Room", Repo: "repo", DefaultBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.New(db))
	t.Cleanup(httpServer.Close)

	joined, err := client.Join(context.Background(), httpServer.URL, room.JoinCode, "Abby", "Claude Code")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	api := &client.API{BaseURL: httpServer.URL, Token: joined.Token}
	snapshot, err := api.Snapshot(context.Background(), room.ID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if joined.Participant.ID == "" || len(snapshot.Participants) != 1 {
		t.Fatalf("join=%#v snapshot=%#v", joined, snapshot)
	}
}
