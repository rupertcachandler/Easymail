SHELL := /bin/sh

GO              ?= go
GOLANGCI_LINT   ?= golangci-lint
GOLANGCI_VERSION?= v2.11.4
PKG             ?= ./...
COVER           ?= cover.out
FUZZTIME        ?= 30s

.PHONY: all test race vet lint lint-fix spec-lint cover cover-gate fuzz fuzz-smoke tidy clean tools ci

all: vet lint test cover-gate

# ci mirrors the steps run by .github/workflows/ci.yml so that
# `make ci` failing locally implies the same failure in CI.
ci: tools
	$(GO) mod verify
	$(GO) vet $(PKG)
	$(GOLANGCI_LINT) run $(PKG)
	$(GO) test -race -count=1 -covermode=atomic -coverprofile=$(COVER) $(PKG)
	$(GO) run ./internal/spec/cmd/covergate $(COVER)
	$(GO) run ./internal/spec/cmd/speclint
	$(GO) test ./wbxml -run='^$$' -fuzz=FuzzDecode -fuzztime=$(FUZZTIME)

test:
	$(GO) test -count=1 $(PKG)

race:
	$(GO) test -race -count=1 $(PKG)

vet:
	$(GO) vet $(PKG)

tools:
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
	  echo "installing golangci-lint $(GOLANGCI_VERSION)"; \
	  $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION); \
	}

lint: tools
	$(GOLANGCI_LINT) run $(PKG)

lint-fix: tools
	$(GOLANGCI_LINT) run --fix $(PKG)

spec-lint:
	$(GO) run ./internal/spec/cmd/speclint

cover:
	$(GO) test -race -count=1 -covermode=atomic -coverprofile=$(COVER) $(PKG)
	$(GO) tool cover -func=$(COVER) | tail -n 1

cover-gate: cover
	$(GO) run ./internal/spec/cmd/covergate $(COVER)

fuzz:
	$(GO) test ./wbxml -run='^$$' -fuzz=FuzzDecode -fuzztime=$(FUZZTIME)

tidy:
	$(GO) mod tidy

clean:
	rm -f $(COVER) cover.html
