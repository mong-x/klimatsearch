.PHONY: test test-hosted build build-hosted run models check-models eval-golden e2e docker help tidy

help:
	@echo "make test | build | run | models | check-models | eval-golden | docker | e2e"
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

# go mod tidy walks every build configuration, so hosted-only module deps
# stay in go.mod without tags (Go 1.27 removed -tags from this subcommand).
tidy:
	go mod tidy

run: build
	./bin/klimatsearch --embedder=fake --demo-fixture --listen=:8080

models:
	./scripts/download-models.sh
	./scripts/fetch-libtokenizers.sh

check-models:
	./scripts/check-models.sh

# Honest retrieval metrics. Needs data/klimat.db ingested with F2LLM, not Fake.
eval-golden:
	CGO_ENABLED=1 go run -tags "$(GO_TAGS)" ./cmd/evalgolden --embedder=onnx

docker:
	docker build -t klimatsearch:local .

e2e:
	./scripts/e2e-kind.sh
