# Run `make help` to display help

# --- Global -------------------------------------------------------------------
O = out
COVERAGE = 64
VERSION ?= $(shell git describe --tags --dirty  --always)
REPO_ROOT = $(shell git rev-parse --show-toplevel)

all: build test check-coverage lint  ## build, test, check coverage and lint
	@if [ -e .git/rebase-merge ]; then git --no-pager log -1 --pretty='%h %s'; fi
	@echo '$(COLOUR_GREEN)Success$(COLOUR_NORMAL)'

ci: clean check-testdata all  ## Full clean build and up-to-date checks as run on CI

clean::  ## Remove generated files
	-rm -rf $(O)

.PHONY: all ci clean

# --- Build --------------------------------------------------------------------
GO_LDFLAGS = -X main.version=$(VERSION)

build: | $(O)  ## Build reflect binaries
	go build -o $(O)/protog -ldflags='$(GO_LDFLAGS)' .

install:  ## Build and install binaries in $GOBIN
	go install -ldflags='$(GO_LDFLAGS)' .

.PHONY: build install

# --- Test ---------------------------------------------------------------------
COVERFILE = $(O)/coverage.txt

test: ## Run tests and generate a coverage file
	go test -coverprofile=$(COVERFILE) ./...

check-coverage: test  ## Check that test coverage meets the required level
	@go tool cover -func=$(COVERFILE) | $(CHECK_COVERAGE) || $(FAIL_COVERAGE)

cover: test  ## Show test coverage in your browser
	go tool cover -html=$(COVERFILE)

check-testdata: tools
	protoc -o testdata/001-protoc.pb testdata/001.proto
	reflect fdsf testdata/001-protoc.pb -f json -o testdata/001-protoc.json
	test -z "$$(git status --porcelain testdata)"

CHECK_COVERAGE = awk -F '[ \t%]+' '/^total:/ {print; if ($$3 < $(COVERAGE)) exit 1}'
FAIL_COVERAGE = { echo '$(COLOUR_RED)FAIL - Coverage below $(COVERAGE)%$(COLOUR_NORMAL)'; exit 1; }

.PHONY: check-coverage check-testdata cover test

# --- Lint ---------------------------------------------------------------------

lint:  ## Lint go source code
	golangci-lint run

.PHONY: lint

# --- Release -------------------------------------------------------------------
NEXTTAG := $(shell { git tag --list --merged HEAD --sort=-v:refname; echo v0.0.0; } | grep -E "^v?[0-9]+.[0-9]+.[0-9]+$$" | head -n1 | awk -F . '{ print $$1 "." $$2 "." $$3 + 1 }')

release: ## Tag and release binaries for different OS on GitHub release
	git tag $(NEXTTAG)
	git push origin $(NEXTTAG)
	goreleaser release --rm-dist

.PHONY: release

# --- Utilities ----------------------------------------------------------------
COLOUR_NORMAL = $(shell tput sgr0 2>/dev/null)
COLOUR_RED    = $(shell tput setaf 1 2>/dev/null)
COLOUR_GREEN  = $(shell tput setaf 2 2>/dev/null)
COLOUR_WHITE  = $(shell tput setaf 7 2>/dev/null)

help:
	@awk -F ':.*## ' 'NF == 2 && $$1 ~ /^[A-Za-z0-9%_-]+$$/ { printf "$(COLOUR_WHITE)%-25s$(COLOUR_NORMAL)%s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

tools:
	go install github.com/juliaogris/reflect@v0.0.22

$(O):
	@mkdir -p $@

.PHONY: help