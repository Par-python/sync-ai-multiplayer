// Package local stores repository-local Syncroom enrollment configuration.
package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the non-source state needed by every CLI command after attach.
type Config struct {
	ServerURL     string `json:"serverUrl"`
	RoomID        string `json:"roomId"`
	ParticipantID string `json:"participantId"`
	Token         string `json:"token"`
}

// Write atomically persists config with owner-only permissions.
func Write(root string, config Config) error {
	dir := filepath.Join(root, ".syncroom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(bytes); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "config.json"))
}

// Read loads the attached repository configuration.
func Read(root string) (Config, error) {
	bytes, err := os.ReadFile(filepath.Join(root, ".syncroom", "config.json"))
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		return Config{}, fmt.Errorf("parse Syncroom config: %w", err)
	}
	if config.ServerURL == "" || config.RoomID == "" || config.ParticipantID == "" || config.Token == "" {
		return Config{}, fmt.Errorf("incomplete Syncroom config")
	}
	return config, nil
}
