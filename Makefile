# Build wrapper for rtm-cli-go.
#
# Any target that has to fetch spec.json live needs RTM_API_KEY and
# RTM_API_SECRET in the environment. A pre-existing spec.json skips
# the fetch entirely.

BINARY ?= rtm
SPEC   ?= spec.json

.PHONY: build generate spec check

build: $(SPEC)
	go generate ./...
	go build -o $(BINARY) ./cmd/rtm

generate:
	go generate ./...

spec $(SPEC):
	@[ -n "$(RTM_API_KEY)" ] && [ -n "$(RTM_API_SECRET)" ] \
		|| { echo "error: RTM_API_KEY and RTM_API_SECRET must be set to fetch the spec" >&2; exit 1; }
	go tool rtm-gen spec --key=$(RTM_API_KEY) --secret=$(RTM_API_SECRET) --out=$(SPEC)

check: $(SPEC)
	go generate ./...
	go build ./...
	go test -race ./...
	go tool golangci-lint run
	go mod tidy -diff
