# Run `make help` to display help

# --- Global -------------------------------------------------------------------
O = out
COVERAGE = 90
VERSION ?= $(shell git describe --tags --dirty  --always)
REPO_ROOT = $(shell git rev-parse --show-toplevel)

all: build test check-coverage lint  ## build, test, check coverage and lint
	@if [ -e .git/rebase-merge ]; then git --no-pager log -1 --pretty='%h %s'; fi
	@echo '$(COLOUR_GREEN)Success$(COLOUR_NORMAL)'

ci: clean check-uptodate all  ## Full clean build and up-to-date checks as run on CI

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

gen-pb = protoc -o $(1:%.proto=%-protoc.pb) $(1)
gen-json = reflect fdsf $(1:%.proto=%-protoc.pb) -f json | jq . > $(1:%.proto=%-protoc.json)
gen-testdata = $(call gen-pb,$(1))$(nl)$(call gen-json,$(1))$(nl)

gen-testdata:
	$(foreach proto,$(wildcard testdata/*.proto),$(call gen-testdata,$(proto)))
	protosync --dest registry/testdata google/api/annotations.proto
	protoc --include_imports -I registry/testdata -o registry/testdata/regtest.pb registry/testdata/regtest.proto

check-uptodate: gen-testdata protos
	test -z "$$(git status --porcelain)" || { git diff; false; }

CHECK_COVERAGE = awk -F '[ \t%]+' '/^total:/ {print; if ($$3 < $(COVERAGE)) exit 1}'
FAIL_COVERAGE = { echo '$(COLOUR_RED)FAIL - Coverage below $(COVERAGE)%$(COLOUR_NORMAL)'; exit 1; }

.PHONY: check-coverage check-uptodate cover test

# --- Lint ---------------------------------------------------------------------

lint:  ## Lint go source code
	golangci-lint run

.PHONY: lint

# --- Protos ---------------------------------------------------------------------

protos:
	protoc \
		--go_out=. --go_opt=module=foxygo.at/protog  \
		--go-grpc_out=. --go-grpc_opt=module=foxygo.at/protog \
		-I httprule/internal \
		test.proto echo.proto
	gosimports -w .

.PHONY: protos

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

$(O):
	@mkdir -p $@

.PHONY: help

define nl


endef
ifndef ACTIVE_HERMIT
$(eval $(subst \n,$(nl),$(shell bin/hermit env -r | sed 's/^\(.*\)$$/export \1\\n/')))
endif

# Ensure make version is gnu make 3.82 or higher
ifeq ($(filter undefine,$(value .FEATURES)),)
$(error Unsupported Make version. \
	$(nl)Use GNU Make 3.82 or higher (current: $(MAKE_VERSION)). \
	$(nl)Activate 🐚 hermit with `. bin/activate-hermit` and run again \
	$(nl)or use `bin/make`)
endif
