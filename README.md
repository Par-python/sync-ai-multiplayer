# Syncroom

CLI-first coordination layer for developers using different coding agents on
separate laptops. One Go binary runs either as the coordinator (`syncroom
serve`) or as a repository-local participant client.

See:

- `docs/superpowers/specs/2026-07-18-syncroom-go-core-design.md`
- `docs/superpowers/plans/2026-07-18-syncroom-go-core.md`

## Status

Tasks 1–3 of the plan (module + domain contracts, SQLite store, and
authenticated coordinator API with SSE) are the current foundation. Later
tasks (attach, watch, checkpoint, integration, failure routing) are not
implemented yet.

## Development

Requires Go 1.26 or newer.

```bash
gofmt -w cmd internal
go test ./...
go vet ./...
```
