// Package domain contains the shared value types Syncroom exchanges between
// the coordinator, the clients, and the store. Fields are annotated with the
// exact JSON tag used on the wire so store and server code stay in sync.
package domain

import "time"

// IntentStatus enumerates the declared execution state of a participant's
// intent. Values are lowercase strings so they round-trip through JSON and
// SQLite text columns without conversion.
type IntentStatus string

const (
	IntentStatusPlanning  IntentStatus = "planning"
	IntentStatusExecuting IntentStatus = "executing"
	IntentStatusBlocked   IntentStatus = "blocked"
	IntentStatusDone      IntentStatus = "done"
)

// IntegrationRunStatus enumerates the lifecycle of a coordinator integration
// job. The coordinator only ever runs one at a time.
type IntegrationRunStatus string

const (
	IntegrationRunQueued    IntegrationRunStatus = "queued"
	IntegrationRunRunning   IntegrationRunStatus = "running"
	IntegrationRunSucceeded IntegrationRunStatus = "succeeded"
	IntegrationRunFailed    IntegrationRunStatus = "failed"
)

// Room is the top-level coordination unit. A room lives on exactly one
// coordinator; its ID is the durable server-assigned identifier and its
// join code is the short secret used once to enroll new participants.
type Room struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Repo          string    `json:"repo"`
	DefaultBranch string    `json:"defaultBranch"`
	JoinCode      string    `json:"joinCode"`
	CheckCommand  string    `json:"checkCommand,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Participant is a developer/agent attached to a room. TokenHash is the
// SHA-256 of the raw 32-byte bearer token; the raw token is returned exactly
// once, at enrollment.
type Participant struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"roomId"`
	Name      string    `json:"name"`
	Agent     string    `json:"agent"`
	TokenHash string    `json:"-"`
	JoinedAt  time.Time `json:"joinedAt"`
}

// Intent is a participant's declared scope for a task. ExpectedPaths are
// repo-relative and used to compute cross-participant overlaps.
type Intent struct {
	ID            string       `json:"id"`
	RoomID        string       `json:"roomId"`
	ParticipantID string       `json:"participantId"`
	Task          string       `json:"task"`
	Objective     string       `json:"objective"`
	ExpectedPaths []string     `json:"expectedPaths"`
	Status        IntentStatus `json:"status"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

// Checkpoint records a validated, pushed Git checkpoint published by a
// participant. Syncroom never receives the source files themselves.
type Checkpoint struct {
	ID            string    `json:"id"`
	RoomID        string    `json:"roomId"`
	ParticipantID string    `json:"participantId"`
	Branch        string    `json:"branch"`
	CommitSHA     string    `json:"commitSha"`
	Message       string    `json:"message"`
	Summary       string    `json:"summary,omitempty"`
	ChangedPaths  []string  `json:"changedPaths"`
	CreatedAt     time.Time `json:"createdAt"`
}

// IntegrationRun is the coordinator-owned attempt at composing several
// checkpoints in a disposable worktree. Only later tasks (4+) populate the
// output/routing fields; they are declared here to keep the wire contract
// stable.
type IntegrationRun struct {
	ID            string               `json:"id"`
	RoomID        string               `json:"roomId"`
	Status        IntegrationRunStatus `json:"status"`
	CheckpointIDs []string             `json:"checkpointIds"`
	StartedAt     *time.Time           `json:"startedAt,omitempty"`
	FinishedAt    *time.Time           `json:"finishedAt,omitempty"`
	Output        string               `json:"output,omitempty"`
	FailedOwners  []string             `json:"failedOwners,omitempty"`
}

// Event is one entry in the append-only room log. Sequence is the strictly
// increasing per-room ID used for SSE Last-Event-ID replay.
type Event struct {
	Sequence  int64     `json:"sequence"`
	RoomID    string    `json:"roomId"`
	Name      string    `json:"name"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"createdAt"`
}

// Snapshot is the projection returned by the coordinator to bring a client
// (or observer) up to the current room state without replaying events. All
// slices are non-nil (possibly empty) so callers can range without a nil
// check.
type Snapshot struct {
	Room            Room             `json:"room"`
	Participants    []Participant    `json:"participants"`
	Intents         []Intent         `json:"intents"`
	Checkpoints     []Checkpoint     `json:"checkpoints"`
	IntegrationRuns []IntegrationRun `json:"integrationRuns"`
	LatestSequence  int64            `json:"latestSequence"`
}
