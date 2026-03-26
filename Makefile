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

# eval runs the evaluation CLI — pass ARGS to forward flags, e.g.:
#   make eval ARGS="--url https://go.dev"
#   make eval ARGS="--url https://go.dev --ground-truth https://go.dev/llms.txt"
#   make eval ARGS="--url https://go.dev --ground-truth https://go.dev/llms.txt --llm-judge"
eval:
	go run ./tools/eval --env .env $(ARGS)

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
