# ── Variables ────────────────────────────────────────────────────────────────

MODULE := github.com/blueai2022/nucleus
CMD    := ./cmd/nucleus
BIN    := ./bin/nucleus
BUF    := buf

.PHONY: all generate lint-proto breaking build run tidy deps test vet clean

# ── Default ───────────────────────────────────────────────────────────────────

all: generate build

# ── Code generation ───────────────────────────────────────────────────────────

generate:
	$(BUF) generate

lint-proto:
	$(BUF) lint

breaking:
	$(BUF) breaking --against '.git#branch=main'

# ── Build ─────────────────────────────────────────────────────────────────────

build:
	go build -o $(BIN) $(CMD)

run:
	go run $(CMD)/...

# ── Dependencies ──────────────────────────────────────────────────────────────

tidy:
	go mod tidy

deps: tidy
	go mod download

# ── Quality ───────────────────────────────────────────────────────────────────

test:
	go test ./...

vet:
	go vet ./...

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	rm -rf $(BIN) pkg/nucleus