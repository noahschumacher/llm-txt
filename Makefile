.PHONY: run build test tidy confirm

_TITLE := "\033[32m[%s]\033[0m %s\n"
_ERROR := "\033[31m[%s]\033[0m %s\n"

BINARY  = bin/llm-txt
COMMIT  = $(shell git rev-parse --short=12 HEAD)

# ------------------------------------------------------------------------------
# Local

run:
	LLM_TXT_ENV_FILE=.env go run .

build:
	go build -o $(BINARY) .

test:
	go test ./...

# tidy updates go.mod and re-vendors all dependencies
tidy:
	go mod tidy && go mod vendor

# ------------------------------------------------------------------------------
# Helpers

confirm:
	@if [[ -z "$(CI)" ]]; then \
		REPLY="" ; \
		read -p "⚠ Are you sure? [y/n] > " -r ; \
		if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then \
			printf $(_ERROR) "KO" "Stopping" ; \
			exit 1 ; \
		else \
			printf $(_TITLE) "OK" "Continuing" ; \
			exit 0 ; \
		fi \
	fi
