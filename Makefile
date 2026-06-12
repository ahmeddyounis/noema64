GO ?= go
NPM ?= npm

.PHONY: test frontend-smoke uci-smoke bench-random build-uci build-cli build-bench build-gui fmt

fmt:
	$(GO)fmt -w ./cmd ./internal

test:
	$(GO) test ./...
	$(NPM) --prefix cmd/noema64-gui/frontend test

frontend-smoke:
	$(NPM) --prefix cmd/noema64-gui/frontend test

uci-smoke:
	printf 'uci\nisready\nucinewgame\nposition startpos moves e2e4 e7e5 g1f3\ngo movetime 100\nquit\n' | $(GO) run ./cmd/noema64-uci

bench-random:
	$(GO) run ./cmd/noema64-bench -games 100

build-uci:
	$(GO) build -o bin/noema64-uci ./cmd/noema64-uci

build-cli:
	$(GO) build -o bin/noema64 ./cmd/noema64

build-bench:
	$(GO) build -o bin/noema64-bench ./cmd/noema64-bench

build-gui:
	$(GO) build -o bin/noema64-gui ./cmd/noema64-gui
