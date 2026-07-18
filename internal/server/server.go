// Package server exposes the Syncroom coordinator's authenticated JSON and
// Server-Sent Events API. It deliberately contains no repository logic: the
// store owns durable state while later command packages call this API.
package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/syncroom/syncroom/internal/domain"
	"github.com/syncroom/syncroom/internal/store"
)

// Server is an http.Handler backed by one coordinator store.
type Server struct {
	store *store.Store
	mu    sync.Mutex
	subs  map[string]map[chan domain.Event]struct{}
}

// New creates a coordinator HTTP handler.
func New(s *store.Store) *Server {
	return &Server{store: s, subs: make(map[string]map[chan domain.Event]struct{})}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/rooms" {
		s.createRoom(w, r)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "rooms" && parts[3] == "participants" && r.Method == http.MethodPost {
		s.joinRoom(w, r, parts[2])
		return
	}
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "rooms" && r.Method == http.MethodGet {
		roomID := parts[2]
		participant, ok := s.authorizeRoom(w, r, roomID)
		if !ok {
			return
		}
		_ = participant
		switch parts[3] {
		case "snapshot":
			s.snapshot(w, r, roomID)
		case "events":
			s.events(w, r, roomID)
		default:
			writeError(w, http.StatusNotFound, "not found")
		}
		return
	}
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "rooms" && r.Method == http.MethodPost {
		roomID := parts[2]
		participant, ok := s.authorizeRoom(w, r, roomID)
		if !ok {
			return
		}
		switch parts[3] {
		case "intents":
			s.publishIntent(w, r, roomID, participant)
		case "decisions":
			s.publishDecision(w, r, roomID, participant)
		default:
			writeError(w, http.StatusNotFound, "not found")
		}
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (s *Server) publishIntent(w http.ResponseWriter, r *http.Request, roomID string, participant domain.Participant) {
	var input struct {
		Task          string              `json:"task"`
		Objective     string              `json:"objective"`
		ExpectedPaths []string            `json:"expectedPaths"`
		Status        domain.IntentStatus `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status == "" {
		input.Status = domain.IntentStatusPlanning
	}
	intent, err := s.store.PublishIntent(r.Context(), roomID, participant.ID, store.PublishIntentInput{Task: input.Task, Objective: input.Objective, ExpectedPaths: input.ExpectedPaths, Status: input.Status})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishLatest(r.Context(), roomID)
	writeJSON(w, http.StatusCreated, intent)
}

func (s *Server) publishDecision(w http.ResponseWriter, r *http.Request, roomID string, participant domain.Participant) {
	var input struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	decision, err := s.store.PublishDecision(r.Context(), roomID, participant.ID, store.PublishDecisionInput{Title: input.Title, Body: input.Body})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishLatest(r.Context(), roomID)
	writeJSON(w, http.StatusCreated, decision)
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name          string `json:"name"`
		Repo          string `json:"repo"`
		DefaultBranch string `json:"defaultBranch"`
		CheckCommand  string `json:"checkCommand"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	room, err := s.store.CreateRoom(r.Context(), store.CreateRoomInput(input))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func (s *Server) joinRoom(w http.ResponseWriter, r *http.Request, joinCode string) {
	var input struct {
		Name  string `json:"name"`
		Agent string `json:"agent"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if _, err := s.store.LookupRoomByJoinCode(r.Context(), joinCode); err != nil {
		writeStoreError(w, err)
		return
	}
	token, hash, err := newToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	participant, err := s.store.JoinRoom(r.Context(), joinCode, store.JoinRoomInput{
		Name: input.Name, Agent: input.Agent, TokenHash: hash,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishLatest(r.Context(), participant.RoomID)
	writeJSON(w, http.StatusCreated, struct {
		Participant domain.Participant `json:"participant"`
		Token       string             `json:"token"`
	}{Participant: participant, Token: token})
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request, roomID string) {
	snapshot, err := s.store.RoomSnapshot(r.Context(), roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// A join code is an enrollment secret, not shared room metadata. The
	// store returns it for coordinator operations, so strip it at the API
	// boundary before broadcasting a snapshot to participants.
	snapshot.Room.JoinCode = ""
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request, roomID string) {
	after, err := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if r.Header.Get("Last-Event-ID") == "" {
		after, err = 0, nil
	}
	if err != nil || after < 0 {
		writeError(w, http.StatusBadRequest, "invalid Last-Event-ID")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for _, event := range s.replay(r.Context(), roomID, after) {
		if err := writeEvent(w, event); err != nil {
			return
		}
		after = event.Sequence
	}
	flusher.Flush()
	ch := s.subscribe(roomID)
	defer s.unsubscribe(roomID, ch)
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			if event.Sequence <= after {
				continue
			}
			if err := writeEvent(w, event); err != nil {
				return
			}
			after = event.Sequence
			flusher.Flush()
		}
	}
}

func (s *Server) authorizeRoom(w http.ResponseWriter, r *http.Request, roomID string) (domain.Participant, bool) {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) || strings.TrimSpace(strings.TrimPrefix(header, prefix)) == "" {
		writeError(w, http.StatusUnauthorized, "bearer token required")
		return domain.Participant{}, false
	}
	participant, err := s.store.LookupParticipantByTokenHash(r.Context(), tokenHash(strings.TrimSpace(strings.TrimPrefix(header, prefix))))
	if err != nil || participant.RoomID != roomID {
		writeError(w, http.StatusUnauthorized, "invalid bearer token")
		return domain.Participant{}, false
	}
	return participant, true
}

func (s *Server) replay(ctx context.Context, roomID string, after int64) []domain.Event {
	events, err := s.store.EventsAfter(ctx, roomID, after)
	if err != nil {
		return nil
	}
	return events
}

func (s *Server) publishLatest(ctx context.Context, roomID string) {
	events := s.replay(ctx, roomID, 0)
	if len(events) == 0 {
		return
	}
	event := events[len(events)-1]
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs[roomID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Server) subscribe(roomID string) chan domain.Event {
	ch := make(chan domain.Event, 8)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subs[roomID] == nil {
		s.subs[roomID] = make(map[chan domain.Event]struct{})
	}
	s.subs[roomID][ch] = struct{}{}
	return ch
}

func (s *Server) unsubscribe(roomID string, ch chan domain.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs[roomID], ch)
	if len(s.subs[roomID]) == 0 {
		delete(s.subs, roomID)
	}
}

func newToken() (raw, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err = rand.Read(bytes); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(bytes)
	return raw, tokenHash(raw), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}

func decodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeEvent(w http.ResponseWriter, event domain.Event) error {
	payload, err := json.Marshal(struct {
		Name      string          `json:"name"`
		Payload   json.RawMessage `json:"payload"`
		CreatedAt string          `json:"createdAt"`
	}{Name: event.Name, Payload: event.Payload, CreatedAt: event.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00")})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: workspace\ndata: %s\n\n", event.Sequence, payload)
	return err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}
