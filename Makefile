# Makefile for mcp-x11-controller

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=mcp-x11-controller

# Environment variables
export GOPATH=/usr/share/gocode:$(HOME)/go
export CGO_ENABLED=1

# Build flags
LDFLAGS=-ldflags "-s -w"

all: build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(LDFLAGS) .

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

run: build
	./$(BINARY_NAME)

deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run with Xvfb for testing
test-xvfb:
	Xvfb :99 -screen 0 1024x768x24 &
	export DISPLAY=:99 && sleep 2 && xterm &
	export DISPLAY=:99 && ./$(BINARY_NAME)

# Development build with debug symbols
dev:
	$(GOBUILD) -o $(BINARY_NAME) -v .

.PHONY: all build clean run deps test-xvfb dev