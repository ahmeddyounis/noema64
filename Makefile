GO ?= go
NPM ?= npm
WAILS ?= wails

.PHONY: test test-race frontend-smoke uci-smoke bench-random trace-validate perft lint license-check release-check gui-dev build-uci build-cli build-bench build-gui fmt

fmt:
	$(GO)fmt -w ./cmd ./internal

lint:
	@files="$$( $(GO)fmt -l ./cmd ./internal )"; if [ -n "$$files" ]; then echo "$$files"; exit 1; fi
	$(GO) vet ./...
	$(NPM) --prefix cmd/noema64-gui/frontend test

test:
	$(GO) test ./...
	$(NPM) --prefix cmd/noema64-gui/frontend test

test-race:
	$(GO) test -race ./...

frontend-smoke:
	$(NPM) --prefix cmd/noema64-gui/frontend test

perft:
	$(GO) test ./internal/chesscore -run Perft

uci-smoke:
	printf 'uci\nisready\nucinewgame\nposition startpos moves e2e4 e7e5 g1f3\ngo movetime 100\nquit\n' | $(GO) run ./cmd/noema64-uci

bench-random:
	$(GO) run ./cmd/noema64-bench -games 100

trace-validate:
	$(GO) test ./internal/storage -run Trace

license-check:
	$(GO) mod download
	$(GO) list -m -json all | $(GO) run ./cmd/noema64-licensecheck

release-check: build-cli build-uci build-bench
	$(GO) run ./cmd/noema64-releasecheck ./bin/noema64 ./bin/noema64-uci ./bin/noema64-bench

gui-dev:
	cd cmd/noema64-gui && $(WAILS) dev

build-uci:
	$(GO) build -o bin/noema64-uci ./cmd/noema64-uci

build-cli:
	$(GO) build -o bin/noema64 ./cmd/noema64

build-bench:
	$(GO) build -o bin/noema64-bench ./cmd/noema64-bench

build-gui:
	$(GO) build -o bin/noema64-gui ./cmd/noema64-gui
