.PHONY: test build run models e2e docker help

help:
	@echo "make test | build | run | models | docker | e2e"
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

build:
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -o bin/klimatsearch ./cmd/klimatsearch

run: build
	./bin/klimatsearch --embedder=fake --demo-fixture --listen=:8080

models:
	./scripts/download-models.sh
	./scripts/fetch-libtokenizers.sh

docker:
	docker build -t klimatsearch:local .

e2e:
	./scripts/e2e-kind.sh
