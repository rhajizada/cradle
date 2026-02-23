.DEFAULT_GOAL := help
VERSION := $(shell \
  tag=$$(git describe --tags --exact-match 2>/dev/null || true); \
  if [ -n "$$tag" ] && echo "$$tag" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
    echo "$$tag" | sed 's/^v//'; \
  else \
    git rev-parse --short HEAD; \
  fi)

CONFIG_BASE := $(if $(XDG_CONFIG_HOME),$(XDG_CONFIG_HOME),$(HOME)/.config)
CONFIG_DIR := $(CONFIG_BASE)/cradle
CONFIG_FILE := $(CONFIG_DIR)/config.yaml

.PHONY: build
## build: Build project
build:
	go build -ldflags "-s -w -X main.Version=$(VERSION)" -o bin/cradle ./cmd/cradle

.PHONY: clean
## clean: Remove previous builds
clean:
	@rm bin/*

.PHONY: mod
## mod: Install dependecies
mod:
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest \
	@go mod download

.PHONY: install
## install: Install cradle
install: build
	@mkdir -p $(HOME)/.local/bin
	@cp ./bin/cradle $(HOME)/.local/bin
	@echo "Installed cradle to '$(HOME)/.local/bin'. Please add '$(HOME)/.local/bin' to your PATH."

.PHONY: config
## config: Install example config to $XDG_CONFIG_HOME/cradle or ~/.config/cradle
config:
	@mkdir -p $(CONFIG_DIR)
	@cp -R ./examples/. $(CONFIG_DIR)/
	@echo "Copied example configuration to '$(CONFIG_DIR)'."

.PHONY: lint
## lint: Lint source code
lint:
	@golangci-lint run

.PHONY: fmt
## fmt: Format source code
fmt:
	@golangci-lint fmt

.PHONY: test
## test: Run tests
test:
	@go tool gotestsum

.PHONY: coverage
## coverage: Generate test coverage report
coverage:
	@go tool gotestsum -- -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

.PHONY: run
## run: Build and run in development mode
run:
	@go run ./cmd/cradle

.phony: pre-commit
## pre-commit: Run pre-commit hooks
pre-commit:
	@pre-commit run --all-files

.PHONY: schema
## schema: Generate configuration schema JSON
schema:
	@go run ./cmd/configschema > configuration.schema.json

.PHONY: uninstall
## uninstall: Uninstall
uninstall:
	@rm $(HOME)/.local/bin/cradle

.PHONY: help
all: help
# help: show help message
help: Makefile
	@echo
	@echo " Choose a command to run in "$(NAME)":"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo
