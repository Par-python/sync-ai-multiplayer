package domain

// Event names used on the append-only room log. Downstream consumers
// (context renderer, integration router, SSE clients) match on these
// values, so they are declared once here.
const (
	EventRoomCreated       = "room.created"
	EventParticipantJoined = "participant.joined"
	EventIntentPublished   = "intent.published"
	EventDecisionAdded     = "decision.added"
	EventCheckpointPushed  = "checkpoint.pushed"
	EventIntegrationQueued = "integration.queued"
	EventIntegrationDone   = "integration.done"
)
