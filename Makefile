VERSION  := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BRANCH   := $(shell git branch --show-current 2>/dev/null || echo "master")
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LD_FLAGS := -s -w						\
			-X main.version=$(VERSION) 	\
			-X main.branch=$(BRANCH) 	\
			-X main.hash=$(GIT_HASH)

TARGET  = out/mininaru
FMT_DIR = bot/ cli/ config/ core/ modules/ server/ util/

.PHONY: all build test install uninstall clean

all: build

build:
	go build -ldflags "$(LD_FLAGS)" -o $(TARGET) ./cli

test:
	@echo "checking code format..."
	@gofmt -l $(FMT_DIR)

	@echo "checking code static error..."
	@go vet ./...

	@echo "running unit test..."
	@go test ./... -v

install: $(TARGET)
	bash ./scripts/binary-install.sh

uninstall:
	bash ./scripts/binary-install.sh --uninstall

clean:
	@rm -f $(TARGET)
	@rm -rf out/
