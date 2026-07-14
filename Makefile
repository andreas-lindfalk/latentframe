.PHONY: build test tidy fmt vet check proto ingest restage-room

# --- Go ---
build: ## compile everything
	go build ./...

# --- proto (buf) ---
proto: ## lint + regenerate Go from proto definitions → gen/go
	buf lint
	buf generate

test: ## run tests
	go test ./...

tidy: ## sync go.mod/go.sum
	go mod tidy

fmt: ## format
	gofmt -w .

vet: ## static checks
	go vet ./...

check: fmt vet test ## format, vet, test

# --- one-room experiment ---
# Run stage 1+2 on a local video, e.g:
#   make ingest VIDEO=experiments/one-room/kitchen.mov
ingest: ## extract keyframes + transcribe a walkthrough video
	go run ./services/ingest --video "$(VIDEO)" --out out --transcribe --lang es

# understand → restage → verify on one room (Claude writes the brief, no manual prompt):
#   make restage-room IN=experiments/one-room/kitchen.jpg ROOM=kitchen
restage-room: ## auto brief → re-stage → honesty gate for one room image
	@mkdir -p out
	@STYLE=$$(go run ./services/director understand --image "$(IN)" --room "$(ROOM)" --print style); \
	echo "style → $$STYLE"; \
	go run ./services/render restage --in "$(IN)" --out out/after.png --room "$(ROOM)" --style "$$STYLE"; \
	go run ./services/director verify --before "$(IN)" --after out/after.png --room "$(ROOM)"
