.PHONY: rtfm test show_latest_tag build run release

VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_REVISION := $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
LDFLAGS := -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitRevision=$(GIT_REVISION)

rtfm:
	cat Makefile

test:
	go test ./...

show_latest_tag:
	@echo Latest tag is $(shell git tag --sort=-v:refname | head -n 1); \

build: test
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o ./hashrouter ./cmd/hashrouter/main.go

run: build
	./hashrouter

release: show_latest_tag
	@while true; do \
		read -p "Enter new tag (vX.Y.Z): " TAG; \
		if echo "$$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
			break; \
		else \
			echo "Invalid tag, try again (vX.Y.Z)"; \
		fi; \
	done; \
	rm -rf dist; \
	sed -i.bak -E "s|(image:[[:space:]]+.*hashrouter:).*|\1$$TAG|g" deploy/deployment.yaml; \
	rm -f deploy/deployment.yaml.bak; \
	git add deploy/deployment.yaml; \
	git commit -m "deployment uses tag $$TAG"; \
	git push; \
	git tag $$TAG; \
	goreleaser release
