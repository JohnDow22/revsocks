VERSION=2.9
GIT_COMMIT = $(shell git rev-parse HEAD | cut -c1-7)
BUILD_OPTIONS = -ldflags "-X github.com/kost/revsocks/internal/common.Version=$(VERSION) -X github.com/kost/revsocks/internal/common.CommitID=$(GIT_COMMIT)"
STATIC_OPTIONS = -ldflags "-extldflags='-static' -X github.com/kost/revsocks/internal/common.Version=$(VERSION) -X github.com/kost/revsocks/internal/common.CommitID=$(GIT_COMMIT)"

# ========================================
# Default: build both binaries
# ========================================
default: agent server

# ========================================
# New Architecture: Separate Binaries
# ========================================

agent: dep
	go build $(BUILD_OPTIONS) -o revsocks-agent ./cmd/agent

server: dep
	go build $(BUILD_OPTIONS) -o revsocks-server ./cmd/server

# Build both with static linking
static: dep
	CGO_ENABLED=0 go build $(STATIC_OPTIONS) -o revsocks-agent ./cmd/agent
	CGO_ENABLED=0 go build $(STATIC_OPTIONS) -o revsocks-server ./cmd/server

# ========================================
# Havoc DllSpawn — c-shared DLL (args из lpvReserved, как ligolo-ng)
# ========================================
# DllMain (cmd/agent/havoc_dll.go) читает base64 args из lpvReserved → mainImpl().
# Сборка под windows/amd64, mingw-w64. -ldflags -s -w для strip (~5-7MB).
agent-dll: dep
	mkdir -p dist
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
		go build -buildmode=c-shared -ldflags "-s -w -X github.com/kost/revsocks/internal/common.Version=$(VERSION)-dll -X github.com/kost/revsocks/internal/common.CommitID=$(GIT_COMMIT)" \
		-o dist/revsocks-agent.dll ./cmd/agent
	@ls -lh dist/revsocks-agent.dll dist/revsocks-agent.h 2>/dev/null || true

# ========================================
# Legacy: Single binary (for compatibility)
# ========================================
revsocks: dep
	go build -ldflags "-X main.Version=$(VERSION) -X main.CommitID=$(GIT_COMMIT)" -o revsocks

dep:
	go mod download

tools:
	go install github.com/mitchellh/gox@latest
	go install github.com/tcnksm/ghr@latest

ver:
	echo version $(VERSION)

gittag:
	git tag v$(VERSION)
	git push --tags origin master

clean:
	rm -rf dist
	rm -f revsocks-agent revsocks-server revsocks
	rm -f *.exe

dist:
	mkdir -p dist

# Cross-compilation for all platforms
gox-agent:
	CGO_ENABLED=0 gox -osarch="!darwin/386" -ldflags="-s -w -X github.com/kost/revsocks/internal/common.Version=$(VERSION) -X github.com/kost/revsocks/internal/common.CommitID=$(GIT_COMMIT)" -output="dist/revsocks-agent_{{.OS}}_{{.Arch}}" ./cmd/agent

gox-server:
	CGO_ENABLED=0 gox -osarch="!darwin/386" -ldflags="-s -w -X github.com/kost/revsocks/internal/common.Version=$(VERSION) -X github.com/kost/revsocks/internal/common.CommitID=$(GIT_COMMIT)" -output="dist/revsocks-server_{{.OS}}_{{.Arch}}" ./cmd/server

gox: gox-agent gox-server

goxwin-agent:
	CGO_ENABLED=0 gox -osarch="windows/amd64 windows/386" -ldflags="-s -w -X github.com/kost/revsocks/internal/common.Version=$(VERSION) -X github.com/kost/revsocks/internal/common.CommitID=$(GIT_COMMIT)" -output="dist/revsocks-agent_{{.OS}}_{{.Arch}}" ./cmd/agent

goxwin-server:
	CGO_ENABLED=0 gox -osarch="windows/amd64 windows/386" -ldflags="-s -w -X github.com/kost/revsocks/internal/common.Version=$(VERSION) -X github.com/kost/revsocks/internal/common.CommitID=$(GIT_COMMIT)" -output="dist/revsocks-server_{{.OS}}_{{.Arch}}" ./cmd/server

goxwin: goxwin-agent goxwin-server

dokbuild:
	docker run -it --rm -v $(PWD):/app golang:alpine /bin/sh -c 'apk add make file git && git config --global --add safe.directory /app && cd /app && make -B tools && make gox && make goxwin'

draft:
	ghr -draft v$(VERSION) dist/

# ========================================
# Stealth Build Targets
# ========================================

# Собрать stealth версию из config.yaml
stealth:
	@echo "Building stealth agent from config.yaml..."
	@bash build_stealth.sh

# Тестировать stealth бинарник
stealth-test:
	@echo "Testing stealth binary..."
	@OUTPUT=$$(grep 'output_name:' config.yaml | awk '{print $$2}' | tr -d '"'); \
	if [ -f "$$OUTPUT" ]; then \
		echo "Binary: $$OUTPUT"; \
		ls -lh "$$OUTPUT"; \
		echo "Testing execution (will exit immediately)..."; \
		timeout 2 ./$$OUTPUT || true; \
		echo "Binary executable"; \
	else \
		echo "Binary not found: $$OUTPUT"; \
		echo "Run 'make stealth' first"; \
		exit 1; \
	fi

# Очистка stealth артефактов
stealth-clean:
	@echo "Cleaning stealth build artifacts..."
	@OUTPUT=$$(grep 'output_name:' config.yaml | awk '{print $$2}' | tr -d '"' 2>/dev/null || echo "sessions"); \
	rm -f $$OUTPUT *.backup
	@echo "Cleaned"

# Помощь по stealth targets
stealth-help:
	@echo "========================================"
	@echo "       RevSocks Stealth Build Help     "
	@echo "========================================"
	@echo ""
	@echo "Available targets:"
	@echo "  make agent          - Build agent binary"
	@echo "  make server         - Build server binary"
	@echo "  make stealth        - Build stealth agent from config.yaml"
	@echo "  make stealth-test   - Test stealth binary"
	@echo "  make stealth-clean  - Clean stealth artifacts"
	@echo ""
	@echo "Quick start:"
	@echo "  1. Edit config.yaml (set server, password, etc.)"
	@echo "  2. make stealth"
	@echo "  3. ./sessions (or your configured output_name)"

# ========================================
# Testing
# ========================================

test:
	go test -v ./...

test-e2e:
	go test -v ./tests/e2e/...

.PHONY: default agent server static agent-dll revsocks dep tools ver gittag clean dist gox gox-agent gox-server goxwin goxwin-agent goxwin-server dokbuild draft stealth stealth-test stealth-clean stealth-help test test-e2e
