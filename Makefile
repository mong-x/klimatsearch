.PHONY: test test-hosted build build-hosted run models e2e docker help tidy

help:
	@echo "make test | build | run | models | docker | e2e"
	@echo "make build-hosted | test-hosted  # Unkey + MPP Guard (the binary we run)"
	@echo "Human launch steps: docs/LAUNCH.md"

# fts5 is a mattn/go-sqlite3 build tag (FTS5 is off by default).
GO_TAGS ?= fts5
TOKENIZERS_DIR := $(CURDIR)/third_party/tokenizers
ifneq ($(wildcard $(TOKENIZERS_DIR)/libtokenizers.a),)
GO_TAGS += tokenizers
export CGO_LDFLAGS += -L$(TOKENIZERS_DIR)
endif

test:
	CGO_ENABLED=1 go test -tags "$(GO_TAGS)" ./...

test-hosted:
	CGO_ENABLED=1 go test -tags "$(GO_TAGS) hosted" ./...

build:
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/klimatsearch ./cmd/klimatsearch

build-hosted:
	CGO_ENABLED=1 go build -tags "$(GO_TAGS) hosted" -o bin/klimatsearch-hosted ./cmd/klimatsearch

tidy:
	go mod tidy -tags "fts5 hosted"

run: build
	./bin/klimatsearch --embedder=fake --demo-fixture --listen=:8080

models:
	./scripts/download-models.sh
	./scripts/fetch-libtokenizers.sh

docker:
	docker build -t klimatsearch:local .

e2e:
	./scripts/e2e-kind.sh
