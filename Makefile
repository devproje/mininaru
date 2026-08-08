VERSION  := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BRANCH   := $(shell git symbolic-ref --short -q HEAD 2>/dev/null || echo "detached")
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DIRTY    := $(shell test -z "$$(git status --porcelain 2>/dev/null)" || echo "-dirty")

LD_FLAGS := -s -w						\
			-X main.version=$(VERSION) 	\
			-X main.branch=$(BRANCH) 	\
			-X main.hash=$(GIT_HASH)$(DIRTY)

TARGET  = out/mininaru
FMT_DIR = bot/ cli/ config/ core/ modules/ server/ util/

COVER_OUT = out/coverage.out

.PHONY: all build fmt vet test test-race test-cover test-all install uninstall clean

all: build

build:
	go build -ldflags "$(LD_FLAGS)" -o $(TARGET) ./cli

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
	@rm -rf out/
