.PHONY: build watch context decisions updates shared

build:
	go build -o ./bin/syncroom ./cmd/syncroom

# Build the current CLI, then keep the local context synchronized in this terminal.
watch: build
	./bin/syncroom watch

context:
	cat .syncroom/context.md

decisions:
	cat .syncroom/decisions.md

updates:
	cat .syncroom/updates.md

shared:
	@cat .syncroom/context.md
	@printf '\n--- Shared decisions ---\n\n'
	@cat .syncroom/decisions.md
	@printf '\n--- Syncroom updates ---\n\n'
	@cat .syncroom/updates.md
