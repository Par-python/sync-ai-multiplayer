package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrations are applied in order the first time a database is opened and
// are idempotent: each statement uses `IF NOT EXISTS`. When new migrations
// are needed, append to the slice; never edit an existing entry.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS rooms (
		id             TEXT PRIMARY KEY,
		name           TEXT NOT NULL,
		repo           TEXT NOT NULL,
		default_branch TEXT NOT NULL,
		join_code      TEXT NOT NULL UNIQUE,
		check_command  TEXT NOT NULL DEFAULT '',
		created_at     TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS participants (
		id         TEXT PRIMARY KEY,
		room_id    TEXT NOT NULL REFERENCES rooms(id),
		name       TEXT NOT NULL,
		agent      TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		joined_at  TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS participants_by_room ON participants(room_id)`,
	`CREATE TABLE IF NOT EXISTS events (
		sequence   INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id    TEXT NOT NULL REFERENCES rooms(id),
		name       TEXT NOT NULL,
		payload    BLOB NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS events_by_room ON events(room_id, sequence)`,
	`CREATE TABLE IF NOT EXISTS intents (
		id             TEXT PRIMARY KEY,
		room_id        TEXT NOT NULL REFERENCES rooms(id),
		participant_id TEXT NOT NULL REFERENCES participants(id),
		task           TEXT NOT NULL,
		objective      TEXT NOT NULL,
		expected_paths TEXT NOT NULL,
		status         TEXT NOT NULL,
		updated_at     TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS decisions (
		id             TEXT PRIMARY KEY,
		room_id        TEXT NOT NULL REFERENCES rooms(id),
		participant_id TEXT NOT NULL,
		title          TEXT NOT NULL,
		body           TEXT NOT NULL,
		created_at     TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS checkpoints (
		id             TEXT PRIMARY KEY,
		room_id        TEXT NOT NULL REFERENCES rooms(id),
		participant_id TEXT NOT NULL REFERENCES participants(id),
		branch         TEXT NOT NULL,
		commit_sha     TEXT NOT NULL,
		message        TEXT NOT NULL,
		summary        TEXT NOT NULL,
		changed_paths  TEXT NOT NULL,
		created_at     TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS integration_runs (
		id             TEXT PRIMARY KEY,
		room_id        TEXT NOT NULL REFERENCES rooms(id),
		status         TEXT NOT NULL,
		checkpoint_ids TEXT NOT NULL,
		started_at     TEXT,
		finished_at    TEXT,
		output         TEXT NOT NULL DEFAULT '',
		failed_owners  TEXT NOT NULL DEFAULT '[]'
	)`,
	`CREATE TABLE IF NOT EXISTS routed_updates (
		id             TEXT PRIMARY KEY,
		room_id        TEXT NOT NULL REFERENCES rooms(id),
		participant_id TEXT NOT NULL,
		payload        BLOB NOT NULL,
		created_at     TEXT NOT NULL
	)`,
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	for i, stmt := range migrations {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	return nil
}
