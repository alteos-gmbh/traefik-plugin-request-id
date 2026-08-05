.PHONY: default test yaegi-test vendor vendor-check clean

MODULE  := github.com/alteos-gmbh/traefik-plugin-request-id
GOPATH  ?= $(shell go env GOPATH)
SRCDIR  := $(GOPATH)/src/$(MODULE)

default: test yaegi-test

test:
	gofmt -l . | grep -v '^vendor/' | (! grep .) || (echo "run gofmt -w on the files above" && exit 1)
	go vet -mod=vendor ./...
	go test -mod=vendor -v -cover ./...

# Traefik loads plugins through the Yaegi interpreter rather than compiling them,
# so a package that builds with the Go toolchain can still fail at runtime.
# Yaegi resolves imports through GOPATH, so the sources have to be reachable at
# $GOPATH/src/$(MODULE). Pass the import path, not ".": the "." form cannot
# resolve the vendored dependencies.
yaegi-test:
	@mkdir -p $(dir $(SRCDIR))
	@if [ ! -e $(SRCDIR) ]; then ln -s $(CURDIR) $(SRCDIR); fi
	GOPATH=$(GOPATH) yaegi test -v $(MODULE)

vendor:
	go mod vendor

# Fails if the committed vendor tree does not match what go mod vendor produces.
vendor-check: vendor
	git diff --exit-code -- go.mod go.sum vendor/

clean:
	rm -rf ./vendor
