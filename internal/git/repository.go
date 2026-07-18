// Package git wraps only the explicit Git commands Syncroom needs. It never
// invokes a shell, so repository paths and user-provided arguments are not
// interpolated into command strings.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// RepositoryState is the lightweight repository metadata the observer publishes.
type RepositoryState struct {
	Root         string
	Branch       string
	Head         string
	Dirty        bool
	ChangedPaths []string
}

// Root returns the containing Git worktree root.
func Root(path string) (string, error) { return run(path, "rev-parse", "--show-toplevel") }

// State reads branch, HEAD, dirty flag, and changed paths from one worktree.
func State(root string) (RepositoryState, error) {
	actualRoot, err := Root(root)
	if err != nil {
		return RepositoryState{}, err
	}
	branch, err := run(actualRoot, "branch", "--show-current")
	if err != nil {
		return RepositoryState{}, err
	}
	head, err := run(actualRoot, "rev-parse", "HEAD")
	if err != nil {
		return RepositoryState{}, err
	}
	status, err := run(actualRoot, "status", "--porcelain")
	if err != nil {
		return RepositoryState{}, err
	}
	paths, err := run(actualRoot, "diff", "--name-only")
	if err != nil {
		return RepositoryState{}, err
	}
	return RepositoryState{Root: actualRoot, Branch: branch, Head: head, Dirty: status != "", ChangedPaths: lines(paths)}, nil
}

// HasOrigin reports whether the worktree has an origin remote.
func HasOrigin(root string) bool {
	_, err := run(root, "remote", "get-url", "origin")
	return err == nil
}

// SwitchNew creates and switches to a new branch in the given worktree.
func SwitchNew(root, branch string) error {
	_, err := run(root, "switch", "-c", branch)
	return err
}

func run(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func lines(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, "\n")
}
