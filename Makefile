# SPDX-License-Identifier: GPL-2.0-or-later
# Copyright (C) 2022-2026 Project_IO

CC = go
MAIN = main.go
SRCS = *.go **/*.go
TARGET = mininaru

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "0.0.0")
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d' ' -f3)

LD_FLAGS := -X main.version=$(VERSION)		\
            -X main.branch=$(BRANCH)		\
            -X main.hash=$(HASH)			\
            -X main.buildTime=$(BUILD_TIME)	\
            -X main.goVersion=$(GO_VERSION)

all: $(SRCS)
	$(CC) build -ldflags "$(LD_FLAGS)" -o $(TARGET) $(MAIN)

clean:
	rm -f $(TARGET)

.PHONY: all clean
