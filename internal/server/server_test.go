package server_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syncroom/syncroom/internal/server"
	"github.com/syncroom/syncroom/internal/store"
)

func TestCreateJoinSnapshot(t *testing.T) {
	api := newTestServer(t)

	room := doJSON(t, api.Client(), http.MethodPost, api.URL+"/v1/rooms", "", map[string]string{
		"name": "Lost & Found", "repo": "https://example.test/found.git", "defaultBranch": "main",
	})
	roomID := stringField(t, room, "id")
	joinCode := stringField(t, room, "joinCode")

	joined := doJSON(t, api.Client(), http.MethodPost, api.URL+"/v1/rooms/"+joinCode+"/participants", "", map[string]string{
		"name": "Alexi", "agent": "Codex",
	})
	token := stringField(t, joined, "token")
	if len(token) < 40 {
		t.Fatalf("token too short: %q", token)
	}
	participant := objectField(t, joined, "participant")
	if got := stringField(t, participant, "roomId"); got != roomID {
		t.Fatalf("participant room ID=%q, want %q", got, roomID)
	}

	snapshot := doJSON(t, api.Client(), http.MethodGet, api.URL+"/v1/rooms/"+roomID+"/snapshot", token, nil)
	snapshotRoom := objectField(t, snapshot, "room")
	if got, _ := snapshotRoom["joinCode"].(string); got != "" {
		t.Fatalf("snapshot exposed join code %q", got)
	}
	participants, ok := snapshot["participants"].([]any)
	if !ok || len(participants) != 1 {
		t.Fatalf("participants=%#v, want one participant", snapshot["participants"])
	}
}

func TestSnapshotRequiresMatchingBearerToken(t *testing.T) {
	api := newTestServer(t)
	room := doJSON(t, api.Client(), http.MethodPost, api.URL+"/v1/rooms", "", map[string]string{
		"name": "Room", "repo": "https://example.test/repo.git", "defaultBranch": "main",
	})
	roomID := stringField(t, room, "id")
	response, err := api.Client().Get(api.URL + "/v1/rooms/" + roomID + "/snapshot")
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func TestSSEReplaysJoinedEvent(t *testing.T) {
	api := newTestServer(t)
	room := doJSON(t, api.Client(), http.MethodPost, api.URL+"/v1/rooms", "", map[string]string{
		"name": "Room", "repo": "https://example.test/repo.git", "defaultBranch": "main",
	})
	roomID := stringField(t, room, "id")
	joinCode := stringField(t, room, "joinCode")
	joined := doJSON(t, api.Client(), http.MethodPost, api.URL+"/v1/rooms/"+joinCode+"/participants", "", map[string]string{
		"name": "Abby", "agent": "Claude Code",
	})
	token := stringField(t, joined, "token")

	request, err := http.NewRequest(http.MethodGet, api.URL+"/v1/rooms/"+roomID+"/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Last-Event-ID", "1")
	response, err := api.Client().Do(request)
	if err != nil {
		t.Fatalf("sse request: %v", err)
	}
	defer response.Body.Close()
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content type=%q", got)
	}
	reader := bufio.NewReader(response.Body)
	var lines []string
	for len(lines) < 3 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE line: %v", err)
		}
		lines = append(lines, line)
	}
	text := strings.Join(lines, "")
	if !strings.Contains(text, "event: workspace") || !strings.Contains(text, "participant.joined") || !strings.Contains(text, "id: ") {
		t.Fatalf("unexpected SSE body: %q", text)
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "syncroom.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return httptest.NewServer(server.New(db))
}

func doJSON(t *testing.T, client *http.Client, method, url, token string, body any) map[string]any {
	t.Helper()
	var input io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		input = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, url, input)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status=%d body=%s", response.StatusCode, body)
	}
	var output map[string]any
	if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return output
}

func stringField(t *testing.T, value map[string]any, key string) string {
	t.Helper()
	got, ok := value[key].(string)
	if !ok || got == "" {
		t.Fatalf("%s=%#v, want non-empty string", key, value[key])
	}
	return got
}

func objectField(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	got, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("%s=%#v, want object", key, value[key])
	}
	return got
}
