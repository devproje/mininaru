VERSION  := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BRANCH   := $(shell git symbolic-ref --short -q HEAD 2>/dev/null || echo "detached")
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DIRTY    := $(shell test -z "$$(git status --porcelain 2>/dev/null)" || echo "-dirty")

LD_FLAGS := -s -w						\
			-X main.version=$(VERSION) 	\
			-X main.branch=$(BRANCH) 	\
			-X main.hash=$(GIT_HASH)$(DIRTY)

TARGET  = out/mininaru
FMT_DIR = bot/ cli/ config/ core/ modules/ rpc/ server/ util/

COVER_OUT = out/coverage.out

DIST_DIR = dist
GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)
DIST_NAME = mininaru_$(GOOS)_$(GOARCH)
DIST_BIN  = $(DIST_DIR)/$(DIST_NAME)/mininaru$(if $(filter windows,$(GOOS)),.exe,)

.PHONY: all build generate fmt vet test test-race test-cover test-all dist install uninstall clean

all: build

build:
	go build -ldflags "$(LD_FLAGS)" -o $(TARGET) ./cli

generate:
	sh ./scripts/generate-proto.sh

dist:
	@mkdir -p $(dir $(DIST_BIN))
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -trimpath -ldflags "$(LD_FLAGS)" -o $(DIST_BIN) ./cli
	@cp LICENSE COPYRIGHT.md README.md $(dir $(DIST_BIN))
	@echo "built $(DIST_BIN)"

fmt:
	@echo "checking code format..."
	@test -z "$$(gofmt -l $(FMT_DIR))" || (gofmt -l $(FMT_DIR); exit 1)

vet:
	@echo "checking code static error..."
	@go vet ./...

test: fmt vet
	@echo "running unit test..."
	@go test ./... -v

test-race: fmt vet
	@echo "running unit test with race detector..."
	@go test -race -count=1 ./... -v

test-cover: fmt vet
	@echo "running unit test with coverage..."
	@mkdir -p out
	@go test -race -count=1 -covermode=atomic -coverprofile=$(COVER_OUT) ./...
	@go tool cover -func=$(COVER_OUT) | tail -n 1

test-all: test-race test-cover

install: build
	bash ./scripts/binary-install.sh

uninstall:
	bash ./scripts/binary-install.sh --uninstall

clean:
	@rm -f $(TARGET) $(COVER_OUT)
	@rm -rf out/ $(DIST_DIR)/
