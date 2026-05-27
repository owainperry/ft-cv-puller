BINARY  := ft-cv-puller
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
DIST    := dist
SHA256  := $(shell command -v shasum >/dev/null 2>&1 && echo "shasum -a 256" || echo "sha256sum")

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build test vet tidy clean install release checksums help

all: build

help:
	@echo "Targets:"
	@echo "  build      build for the host platform -> ./$(BINARY)"
	@echo "  release    cross-compile all platforms + checksums -> ./$(DIST)/"
	@echo "  test       go test ./..."
	@echo "  vet        go vet ./..."
	@echo "  tidy       go mod tidy"
	@echo "  install    go install (host platform)"
	@echo "  clean      remove build artifacts"
	@echo ""
	@echo "VERSION (current): $(VERSION)"

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -rf $(DIST) $(BINARY) $(BINARY).exe

release: clean vet test
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		bin=$(BINARY)$$ext; \
		base=$(BINARY)_$(VERSION)_$${os}_$${arch}; \
		echo "==> $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$$bin . ; \
		if [ "$$os" = "windows" ]; then \
			(cd $(DIST) && zip -q $$base.zip $$bin); \
		else \
			tar -czf $(DIST)/$$base.tar.gz -C $(DIST) $$bin; \
		fi; \
		rm $(DIST)/$$bin; \
	done
	@$(MAKE) --no-print-directory checksums
	@echo ""
	@echo "Release artifacts in $(DIST)/:"
	@ls -1 $(DIST)/

checksums:
	@cd $(DIST) && $(SHA256) *.tar.gz *.zip 2>/dev/null > SHA256SUMS
