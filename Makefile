BINARY := home-proxy
PKG    := github.com/uuigww/home-proxy
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X $(PKG)/internal/version.Version=$(VERSION)

.PHONY: build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

.PHONY: build-deployer
build-deployer:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(BINARY)_darwin_arm64/$(BINARY)  ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(BINARY)_darwin_amd64/$(BINARY)  ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(BINARY)_linux_amd64/$(BINARY)   ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(BINARY)_linux_arm64/$(BINARY)   ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(BINARY)_windows_amd64/$(BINARY).exe ./cmd/$(BINARY)

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	@which golangci-lint >/dev/null 2>&1 || { echo "install golangci-lint: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -rf bin/ dist/

.PHONY: run-local
run-local: build
	./bin/$(BINARY) serve --config ./config.local.toml

.PHONY: help
help:
	@echo "Targets:"
	@echo "  build           - build local binary for current OS/arch into bin/"
	@echo "  build-deployer  - cross-compile deployer for macOS/Linux/Windows into dist/"
	@echo "  test            - run tests"
	@echo "  vet             - go vet"
	@echo "  lint            - golangci-lint"
	@echo "  tidy            - go mod tidy"
	@echo "  clean           - remove build artifacts"
	@echo "  run-local       - build + run serve with ./config.local.toml"
