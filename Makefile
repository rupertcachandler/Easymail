.PHONY: build dev clean install

BINARY=easymail
BUILD_DIR=build
FRONTEND_DIR=frontend
GO?=go
GOFLAGS=-tags webkit2gtk_4_1
LDFLAGS=-s -w

all: build

## build: Build the production binary
build: frontend-build
	CGO_ENABLED=1 GOTOOLCHAIN=auto $(GO) build -tags "webkit2_41 production" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) .

## dev: Start Wails dev mode with hot reload
dev:
	$(HOME)/go/bin/wails dev -tags "webkit2_41 production"

## frontend-build: Build the Vue frontend
frontend-build:
	cd $(FRONTEND_DIR) && npm run build

## frontend-dev: Start the Vite dev server
frontend-dev:
	cd $(FRONTEND_DIR) && npm run dev

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)/$(BINARY)
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(FRONTEND_DIR)/node_modules

## install: Install to /usr/local/bin
install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/

## bindings: Regenerate Wails JS bindings
bindings:
	$(HOME)/go/bin/wails generate module

## deps: Install Go and Node dependencies
deps:
	cd $(FRONTEND_DIR) && npm install
	$(GO) mod download