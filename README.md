# Syncroom

Syncroom is a CLI-first coordination layer for developers using different coding agents on separate laptops. It does not run or inspect the agents: participants attach existing Git clones to one room, declare their intended work, share decisions, and receive generated local context files.

The current MVP stores shared room state in Convex, so laptops do not need to share a LAN or leave a coordinator process running. It supports join-code enrollment, bearer-token participant access, intent and decision synchronization, declared-path overlap detection, and generated agent-readable context. Checkpoint publishing, disposable-worktree integration, and failure routing are planned next.

## Requirements

- Go 1.26 or newer
- Git repository with an `origin` remote for each participant clone
- A Convex account and hosted Convex deployment

## Two-laptop demo

Use this repository as the shared demo repository, or replace its URL with another shared Git remote.

### Create a hosted room

```bash
git clone https://github.com/Par-python/sync-ai-multiplayer.git
cd sync-ai-multiplayer
go build -o ./bin/syncroom ./cmd/syncroom

./bin/syncroom room create \
  --server https://next-mandrill-528.convex.site \
  --name "Syncroom Two-Laptop Demo" \
  --repo "https://github.com/Par-python/sync-ai-multiplayer.git" \
  --default-branch main
```

Copy the printed join code. The Convex site URL is shared over the internet, so neither laptop hosts a server or opens a firewall port.

### Laptop A: attach as Codex

In another terminal:

```bash
cd sync-ai-multiplayer
git switch -c syncroom/alexi/auth

./bin/syncroom attach \
  --server https://next-mandrill-528.convex.site \
  --room YOUR_JOIN_CODE \
  --name Alexi \
  --agent Codex

./bin/syncroom watch
```

Leave `watch` running in its own terminal. In another terminal in the same clone, publish an intent:

```bash
./bin/syncroom intent \
  --task "Authentication" \
  --objective "Add the OAuth callback integration" \
  --areas "internal/server,internal/client" \
  --status executing
```

### Laptop B: attach as Claude Code

Clone, build, and create a separate branch:

```bash
git clone https://github.com/Par-python/sync-ai-multiplayer.git
cd sync-ai-multiplayer
go build -o ./bin/syncroom ./cmd/syncroom
git switch -c syncroom/abby/onboarding
```

Attach to Laptop A's room, then leave the watcher running:

```bash
./bin/syncroom attach \
  --server https://next-mandrill-528.convex.site \
  --room YOUR_JOIN_CODE \
  --name Abby \
  --agent "Claude Code"

./bin/syncroom watch
```

Publish deliberately overlapping work:

```bash
./bin/syncroom intent \
  --task "Onboarding" \
  --objective "Connect the user flow to authentication" \
  --areas "internal/client,internal/context" \
  --status planning
```

### Publish and observe a decision

On either laptop:

```bash
./bin/syncroom decision add \
  --title "OAuth callback route" \
  --body "Use /auth/callback. Read this decision before changing authentication or onboarding."
```

After the next watcher poll (about two seconds), inspect the generated files on either laptop:

```bash
cat .syncroom/context.md
cat .syncroom/decisions.md
cat .syncroom/updates.md
```

The files show both participants, their declared tasks, the shared decision, and the expected-path overlap. In each coding-agent session, add this standing instruction:

```text
Read .syncroom/context.md, .syncroom/decisions.md, and .syncroom/updates.md before planning.
```

## Development

```bash
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
go build -o ./bin/syncroom ./cmd/syncroom
```
