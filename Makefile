VERSION ?= dev
BETTERLEAKS_DIR := .cache/betterleaks/$(shell scripts/platform.sh)

.PHONY: build test lint betterleaks release npm docker clean

## build: build ngcheck for this machine into bin/, next to the pinned Betterleaks
build: betterleaks
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/ngcheck ./cmd/ngcheck
	cp $(BETTERLEAKS_DIR)/betterleaks bin/betterleaks

## test: run all tests against the pinned Betterleaks
test: betterleaks
	NGCHECK_BETTERLEAKS=$(CURDIR)/$(BETTERLEAKS_DIR)/betterleaks go test ./...

## lint: check formatting and run go vet
lint:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "Run: gofmt -w ."; exit 1; }
	go vet ./...

## betterleaks: download the pinned Betterleaks for this machine
betterleaks:
	scripts/fetch-betterleaks.sh host $(BETTERLEAKS_DIR)

## release: build the release archives into dist/ (VERSION=1.2.3)
release:
	scripts/build-release.sh $(VERSION)

## npm: build the npm packages from the release archives into dist/npm/ (VERSION=1.2.3)
npm:
	scripts/build-npm.sh $(VERSION)

## docker: build the Docker image for this machine
docker:
	docker build --build-arg VERSION=$(VERSION) -t ghcr.io/netgen/ngcheck:$(VERSION) .

clean:
	rm -rf bin dist .cache
