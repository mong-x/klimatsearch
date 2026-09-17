.PHONY: test build run models e2e docker

# fts5 is a mattn/go-sqlite3 build tag (FTS5 is off by default).
GO_TAGS ?= fts5

test:
	CGO_ENABLED=1 go test -tags $(GO_TAGS) ./...

build:
	CGO_ENABLED=1 go build -tags $(GO_TAGS) -o bin/klimatsearch ./cmd/klimatsearch

run: build
	./bin/klimatsearch --embedder=fake --demo-fixture --listen=:8080

models:
	./scripts/download-models.sh

docker:
	docker build -t klimatsearch:local .

e2e:
	./scripts/e2e-kind.sh
