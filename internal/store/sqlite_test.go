package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/syncroom/syncroom/internal/domain"
	"github.com/syncroom/syncroom/internal/store"
)

func TestRoomJoinAndSnapshot(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "room.db")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	})

	room, err := s.CreateRoom(ctx, store.CreateRoomInput{
		Name:          "acme",
		Repo:          "git@github.com:acme/repo.git",
		DefaultBranch: "main",
		CheckCommand:  "go test ./...",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if room.JoinCode == "" || room.ID == "" {
		t.Fatalf("expected identifiers to be populated, got %#v", room)
	}

	joined, err := s.JoinRoom(ctx, room.JoinCode, store.JoinRoomInput{
		Name:      "alexi",
		Agent:     "claude-code",
		TokenHash: "hash-a",
	})
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if joined.RoomID != room.ID || joined.Name != "alexi" {
		t.Fatalf("unexpected participant: %#v", joined)
	}

	payload, _ := json.Marshal(map[string]string{"note": "hello"})
	evt, err := s.AppendEvent(ctx, domain.Event{
		RoomID:  room.ID,
		Name:    "participant.joined",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if evt.Sequence == 0 {
		t.Fatalf("expected assigned sequence, got %d", evt.Sequence)
	}

	snap, err := s.RoomSnapshot(ctx, room.ID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Participants) != 1 || snap.Participants[0].ID != joined.ID {
		t.Fatalf("participants=%v", snap.Participants)
	}
	if snap.LatestSequence != evt.Sequence {
		t.Fatalf("latest=%d evt=%d", snap.LatestSequence, evt.Sequence)
	}
}

func TestJoinWithWrongCodeReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "room.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if _, err := s.CreateRoom(ctx, store.CreateRoomInput{Name: "a", Repo: "r", DefaultBranch: "main"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = s.JoinRoom(ctx, "not-a-real-code", store.JoinRoomInput{Name: "n", Agent: "a", TokenHash: "h"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEventsAfterReplaysInOrder(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "room.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	room, err := s.CreateRoom(ctx, store.CreateRoomInput{Name: "r", Repo: "r", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var seqs []int64
	for _, n := range []string{"a", "b", "c"} {
		evt, err := s.AppendEvent(ctx, domain.Event{RoomID: room.ID, Name: n, Payload: []byte("{}")})
		if err != nil {
			t.Fatalf("append: %v", err)
		}
		seqs = append(seqs, evt.Sequence)
	}
	got, err := s.EventsAfter(ctx, room.ID, seqs[0])
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(got) != 2 || got[0].Name != "b" || got[1].Name != "c" {
		t.Fatalf("unexpected events: %#v", got)
	}
	all, err := s.EventsAfter(ctx, room.ID, 0)
	if err != nil {
		t.Fatalf("events all: %v", err)
	}
	if len(all) < 4 || all[0].Name != domain.EventRoomCreated {
		t.Fatalf("expected room.created first, got %#v", all)
	}
}

func TestReopenPreservesData(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "room.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	room, err := s.CreateRoom(ctx, store.CreateRoomInput{Name: "n", Repo: "r", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	got, err := s2.RoomSnapshot(ctx, room.ID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if got.Room.ID != room.ID {
		t.Fatalf("mismatch: %#v", got.Room)
	}
}
