// Package client contains the small authenticated transport used by the
// repository-local CLI and observer. It has no persistence or Git knowledge.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/syncroom/syncroom/internal/domain"
)

// API is a client for one coordinator and one participant token.
type API struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// Snapshot fetches the current room projection using the participant token.
func (a *API) Snapshot(ctx context.Context, roomID string) (domain.Snapshot, error) {
	var snapshot domain.Snapshot
	if err := a.request(ctx, http.MethodGet, "/v1/rooms/"+roomID+"/snapshot", nil, &snapshot); err != nil {
		return domain.Snapshot{}, err
	}
	return snapshot, nil
}

func (a *API) request(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.BaseURL, "/")+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+a.Token)
	request.Header.Set("Accept", "application/json")
	if in != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	httpClient := a.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("coordinator returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	if out != nil {
		if err := json.NewDecoder(response.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
