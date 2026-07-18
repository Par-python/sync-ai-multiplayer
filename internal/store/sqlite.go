// Package store owns Syncroom's SQLite-backed persistence: rooms,
// participants, the append-only event log, and the materialized state used
// to build snapshots. All exported methods are safe for concurrent use;
// SQLite is opened in WAL mode with foreign keys enabled and every mutation
// runs inside a transaction that appends its event alongside the state
// change.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/syncroom/syncroom/internal/domain"
)

// ErrNotFound is returned when a room/participant/event is looked up by a
// key that does not exist. Callers should compare with errors.Is.
var ErrNotFound = errors.New("store: not found")

// Store wraps a single SQLite database.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// Open opens (or creates) the SQLite database at path and applies pending
// migrations. WAL mode and foreign_keys are enabled once at connection
// time via query parameters.
func Open(path string) (*Store, error) {
	// modernc.org/sqlite accepts standard PRAGMA-as-query-parameter syntax,
	// which lets us configure WAL + foreign_keys without racing an initial
	// PRAGMA statement.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", url.PathEscape(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	// SQLite really only serves one writer at a time; a small pool keeps
	// tests deterministic and avoids "database is locked" flakes.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if err := applyMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// CreateRoomInput is the caller-supplied portion of a new room. IDs, join
// codes, and timestamps are assigned by the store.
type CreateRoomInput struct {
	Name          string
	Repo          string
	DefaultBranch string
	CheckCommand  string
}

// CreateRoom inserts a room with a freshly generated ID and join code, and
// appends a room.created event in the same transaction.
func (s *Store) CreateRoom(ctx context.Context, in CreateRoomInput) (domain.Room, error) {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Repo) == "" || strings.TrimSpace(in.DefaultBranch) == "" {
		return domain.Room{}, fmt.Errorf("store: name, repo, and default branch are required")
	}
	room := domain.Room{
		ID:            newID("room"),
		Name:          in.Name,
		Repo:          in.Repo,
		DefaultBranch: in.DefaultBranch,
		JoinCode:      newJoinCode(),
		CheckCommand:  in.CheckCommand,
		CreatedAt:     s.now(),
	}
	payload, err := json.Marshal(struct {
		Room domain.Room `json:"room"`
	}{Room: room})
	if err != nil {
		return domain.Room{}, fmt.Errorf("marshal room event: %w", err)
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rooms (id, name, repo, default_branch, join_code, check_command, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			room.ID, room.Name, room.Repo, room.DefaultBranch, room.JoinCode, room.CheckCommand, room.CreatedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		return appendEventTx(ctx, tx, room.ID, domain.EventRoomCreated, payload, s.now())
	})
	if err != nil {
		return domain.Room{}, err
	}
	return room, nil
}

// JoinRoomInput carries the caller-provided fields for a JoinRoom call.
// TokenHash must be the SHA-256 of the raw participant token computed by
// the server — the store never sees the raw token.
type JoinRoomInput struct {
	Name      string
	Agent     string
	TokenHash string
}

// JoinRoom locates a room by its join code, inserts a participant, and
// appends the participant.joined event.
func (s *Store) JoinRoom(ctx context.Context, joinCode string, in JoinRoomInput) (domain.Participant, error) {
	if strings.TrimSpace(joinCode) == "" {
		return domain.Participant{}, ErrNotFound
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Agent) == "" || strings.TrimSpace(in.TokenHash) == "" {
		return domain.Participant{}, fmt.Errorf("store: name, agent, and tokenHash are required")
	}
	var p domain.Participant
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var roomID string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM rooms WHERE join_code = ?`, joinCode).Scan(&roomID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		p = domain.Participant{
			ID:        newID("part"),
			RoomID:    roomID,
			Name:      in.Name,
			Agent:     in.Agent,
			TokenHash: in.TokenHash,
			JoinedAt:  s.now(),
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO participants (id, room_id, name, agent, token_hash, joined_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			p.ID, p.RoomID, p.Name, p.Agent, p.TokenHash, p.JoinedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		payload, err := json.Marshal(struct {
			ParticipantID string `json:"participantId"`
			Name          string `json:"name"`
			Agent         string `json:"agent"`
		}{p.ID, p.Name, p.Agent})
		if err != nil {
			return err
		}
		return appendEventTx(ctx, tx, roomID, domain.EventParticipantJoined, payload, s.now())
	})
	if err != nil {
		return domain.Participant{}, err
	}
	return p, nil
}

// AppendEvent writes an event without an accompanying materialized-state
// change. The sequence and CreatedAt fields on the returned Event are the
// ones assigned by the store.
func (s *Store) AppendEvent(ctx context.Context, e domain.Event) (domain.Event, error) {
	if strings.TrimSpace(e.RoomID) == "" || strings.TrimSpace(e.Name) == "" {
		return domain.Event{}, fmt.Errorf("store: roomID and name are required")
	}
	if len(e.Payload) == 0 {
		e.Payload = []byte("{}")
	}
	now := s.now()
	var seq int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		// Guard against dangling room references before the FK does; the
		// error surface is friendlier and matches the participant path.
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM rooms WHERE id = ?`, e.RoomID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO events (room_id, name, payload, created_at) VALUES (?, ?, ?, ?)`,
			e.RoomID, e.Name, e.Payload, now.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		seq, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return domain.Event{}, err
	}
	e.Sequence = seq
	e.CreatedAt = now
	return e, nil
}

// EventsAfter returns every event for room whose sequence is strictly
// greater than after, ordered by sequence ascending. Used by SSE replay
// when a client reconnects with a Last-Event-ID header.
func (s *Store) EventsAfter(ctx context.Context, roomID string, after int64) ([]domain.Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sequence, room_id, name, payload, created_at FROM events
		 WHERE room_id = ? AND sequence > ?
		 ORDER BY sequence ASC`, roomID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Event
	for rows.Next() {
		var (
			e         domain.Event
			createdAt string
		)
		if err := rows.Scan(&e.Sequence, &e.RoomID, &e.Name, &e.Payload, &createdAt); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			e.CreatedAt = t
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LookupRoomByJoinCode returns the room for a join code, or ErrNotFound.
// Exposed for the HTTP layer so participant enrollment can validate the
// code before generating a token.
func (s *Store) LookupRoomByJoinCode(ctx context.Context, joinCode string) (domain.Room, error) {
	return s.selectRoom(ctx, `WHERE join_code = ?`, joinCode)
}

// LookupRoom returns the room by ID.
func (s *Store) LookupRoom(ctx context.Context, roomID string) (domain.Room, error) {
	return s.selectRoom(ctx, `WHERE id = ?`, roomID)
}

// LookupParticipantByTokenHash finds a participant by the SHA-256 hash of
// their token. Used by the bearer-token middleware.
func (s *Store) LookupParticipantByTokenHash(ctx context.Context, tokenHash string) (domain.Participant, error) {
	var (
		p        domain.Participant
		joinedAt string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, room_id, name, agent, token_hash, joined_at FROM participants WHERE token_hash = ?`,
		tokenHash).Scan(&p.ID, &p.RoomID, &p.Name, &p.Agent, &p.TokenHash, &joinedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Participant{}, ErrNotFound
		}
		return domain.Participant{}, err
	}
	if t, err := time.Parse(time.RFC3339Nano, joinedAt); err == nil {
		p.JoinedAt = t
	}
	return p, nil
}

// RoomSnapshot returns the current materialized state for a room plus the
// latest event sequence. It is safe to call while other writers hold the
// database.
func (s *Store) RoomSnapshot(ctx context.Context, roomID string) (domain.Snapshot, error) {
	room, err := s.LookupRoom(ctx, roomID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	snap := domain.Snapshot{
		Room:            room,
		Participants:    []domain.Participant{},
		Intents:         []domain.Intent{},
		Checkpoints:     []domain.Checkpoint{},
		IntegrationRuns: []domain.IntegrationRun{},
	}
	if err := s.loadParticipants(ctx, roomID, &snap); err != nil {
		return domain.Snapshot{}, err
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sequence), 0) FROM events WHERE room_id = ?`, roomID).
		Scan(&snap.LatestSequence); err != nil {
		return domain.Snapshot{}, err
	}
	return snap, nil
}

func (s *Store) selectRoom(ctx context.Context, where string, arg any) (domain.Room, error) {
	var (
		r         domain.Room
		createdAt string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, repo, default_branch, join_code, check_command, created_at
		 FROM rooms `+where, arg).
		Scan(&r.ID, &r.Name, &r.Repo, &r.DefaultBranch, &r.JoinCode, &r.CheckCommand, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Room{}, ErrNotFound
		}
		return domain.Room{}, err
	}
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		r.CreatedAt = t
	}
	return r, nil
}

func (s *Store) loadParticipants(ctx context.Context, roomID string, snap *domain.Snapshot) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, room_id, name, agent, token_hash, joined_at FROM participants
		 WHERE room_id = ? ORDER BY joined_at ASC`, roomID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			p        domain.Participant
			joinedAt string
		)
		if err := rows.Scan(&p.ID, &p.RoomID, &p.Name, &p.Agent, &p.TokenHash, &joinedAt); err != nil {
			return err
		}
		if t, err := time.Parse(time.RFC3339Nano, joinedAt); err == nil {
			p.JoinedAt = t
		}
		snap.Participants = append(snap.Participants, p)
	}
	return rows.Err()
}

func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func appendEventTx(ctx context.Context, tx *sql.Tx, roomID, name string, payload []byte, at time.Time) error {
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO events (room_id, name, payload, created_at) VALUES (?, ?, ?, ?)`,
		roomID, name, payload, at.Format(time.RFC3339Nano))
	return err
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read on all supported platforms never fails; if it
		// did, we would rather crash than silently emit predictable IDs.
		panic(fmt.Sprintf("store: rand read: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func newJoinCode() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("store: rand read: %v", err))
	}
	return hex.EncodeToString(b[:])
}
