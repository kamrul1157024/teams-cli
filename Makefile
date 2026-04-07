.PHONY: build install clean test fmt tidy help skill-install skill-uninstall

# Binary name
BINARY_NAME=teams-cli
# Version
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
# Build directory
BUILD_DIR=bin
# Source directory
SRC_DIR=teams-cli

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Build flags
LDFLAGS=-ldflags "-s -w"

# Install paths
INSTALL_DIR?=/usr/local/bin
SKILL_DIR=$(HOME)/.claude/skills/teams

# Default target
all: build

## build: Build the binary to bin/
build:
	@mkdir -p $(BUILD_DIR)
	cd $(SRC_DIR) && $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME) .

## install: Build and install to /usr/local/bin (or INSTALL_DIR)
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)/$(BINARY_NAME)"

## clean: Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

## test: Run tests
test:
	cd $(SRC_DIR) && $(GOTEST) -v ./...

## fmt: Format code
fmt:
	$(GOFMT) -s -w $(SRC_DIR)/

## tidy: Tidy and verify dependencies
tidy:
	cd $(SRC_DIR) && $(GOMOD) tidy
	cd $(SRC_DIR) && $(GOMOD) verify

## skill-install: Install the Claude Code skill to ~/.claude/skills/teams/
skill-install:
	@mkdir -p $(SKILL_DIR)
	cp skills/teams/SKILL.md $(SKILL_DIR)/SKILL.md
	@echo "Skill installed to $(SKILL_DIR)"
	@echo "Use /teams in Claude Code to start"

## skill-uninstall: Remove the Claude Code skill
skill-uninstall:
	rm -rf $(SKILL_DIR)
	@echo "Skill removed from $(SKILL_DIR)"

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/ /'
