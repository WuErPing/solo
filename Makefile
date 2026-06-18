BINS    := solo solo-relay solo-cli
OUTPUT  := output
APP_DIR := app
RELAY_NODEJS_DIR := relay-nodejs
DAEMON_PORT := 17612
APP_PORT := 19000

GO_MODULES := protocol cli daemon relay-go
GO_TEST_FLAGS := -short -v -race -count=1 -timeout=10m -tags external_api

.PHONY: all darwin linux clean dev dev-web dev-daemon run-daemon stop stop-all restart ci test test-go test-app typecheck lint $(BINS)

all: darwin linux

darwin: solo solo-relay solo-cli

linux: solo-linux-amd64 solo-relay-linux-amd64 solo-cli-linux-amd64

solo:
	GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT)/darwin/$@ ./daemon

solo-relay:
	GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT)/darwin/$@ ./relay-go/cmd/relay

solo-cli:
	GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT)/darwin/$@ ./cli

solo-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(OUTPUT)/linux/solo ./daemon

solo-relay-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(OUTPUT)/linux/solo-relay ./relay-go/cmd/relay

# Node.js relay (self-hosted, from Solo)
solo-relay-nodejs:
	cd $(RELAY_NODEJS_DIR) && npm ci --ignore-scripts && npm run build:nodejs

solo-relay-nodejs-docker:
	docker build -t solo-relay:latest $(RELAY_NODEJS_DIR)

solo-cli-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(OUTPUT)/linux/solo-cli ./cli

# Development targets

dev-web:
	@echo "Starting Expo web dev server on port $(APP_PORT)..."
	cd $(APP_DIR) && npx expo start --web --port $(APP_PORT)

dev-web-relay:
	@echo "Starting Expo web dev server (relay mode) on port $(APP_PORT)..."
	cd $(APP_DIR) && \
	EXPO_PUBLIC_RELAY_ENDPOINT=106.52.40.152:8080 \
	EXPO_PUBLIC_RELAY_SERVER_ID=75df32ee \
	EXPO_PUBLIC_RELAY_PUBLIC_KEY=LbDipkESA0+8Mzs57k0EnIW8wvFLaZ95MxhOHEqWNXs= \
	npx expo start --web --port $(APP_PORT)

dev-daemon:
	@echo "Starting solo daemon on port $(DAEMON_PORT)..."
	$(OUTPUT)/darwin/solo

run-daemon: solo
	@echo "Starting solo daemon in background on port $(DAEMON_PORT)..."
	$(OUTPUT)/darwin/solo > /tmp/solo-daemon.log 2>&1 &
	@echo "Daemon started, logs at /tmp/solo-daemon.log"

dev: stop solo
	@echo "Starting solo daemon + Expo web dev server..."
	$(OUTPUT)/darwin/solo > /tmp/solo-daemon.log 2>&1 &
	@echo "Daemon PID: $$!"
	@sleep 2
	cd $(APP_DIR) && EXPO_PUBLIC_LOCAL_DAEMON=localhost:$(DAEMON_PORT) npx expo start --web --port $(APP_PORT)

stop:
	@echo "Stopping dev processes..."
	-pkill -f "expo start --web" 2>/dev/null || true
	-pkill -f "output/darwin/solo$$" 2>/dev/null || true
	@sleep 1
	-pkill -9 -f "output/darwin/solo$$" 2>/dev/null || true
	@echo "Done."

stop-all:
	@echo "Deleting all agent sessions..."
	$(OUTPUT)/darwin/solo-cli delete --all

restart: darwin
	@echo "Restarting solo daemon..."
	-pkill -f "output/darwin/solo$$" 2>/dev/null || true
	@sleep 1
	-pkill -9 -f "output/darwin/solo$$" 2>/dev/null || true
	@sleep 1
	@$(OUTPUT)/darwin/solo > /tmp/solo-daemon.log 2>&1 & \
	echo "Daemon started (PID: $$!), logs at /tmp/solo-daemon.log"

# CI targets (mirror GitHub Actions)

ci: lint test typecheck

lint:
	@echo "=== Linting packages/highlight ==="
	cd packages/highlight && npx eslint src/
	@echo "=== Linting app-bridge ==="
	cd app-bridge && npx eslint src/
	@echo "=== Linting app ==="
	cd $(APP_DIR) && npx expo lint --max-warnings 0
	@echo "=== All lint passed ==="

test: test-go test-app

test-go:
	@set -e; \
	for mod in $(GO_MODULES); do \
		echo "=== Testing go/$$mod ==="; \
		(cd $$mod && go test $(GO_TEST_FLAGS) -coverprofile=coverage.out ./...); \
	done
	@echo "=== All Go tests passed ==="

test-app: build-workspace-deps
	@echo "=== Testing packages/highlight ==="
	cd packages/highlight && npm test
	@echo "=== Testing app (unit) ==="
	cd $(APP_DIR) && npm run test -- --coverage --project=unit
	@echo "=== Testing app-bridge ==="
	cd app-bridge && npm test -- --coverage
	@echo "=== All app tests passed ==="

build-workspace-deps:
	@echo "=== Building workspace dependencies ==="
	cd $(APP_DIR) && npm run build:workspace-deps
	@echo "=== Workspace dependencies built ==="

typecheck: build-workspace-deps
	@echo "=== Typechecking packages/highlight ==="
	cd packages/highlight && npx tsc --noEmit
	@echo "=== Typechecking app ==="
	cd $(APP_DIR) && npx tsc --noEmit
	@echo "=== Typechecking app-bridge ==="
	cd app-bridge && npx tsc --noEmit
	@echo "=== All typecheck passed ==="

clean:
	rm -rf $(OUTPUT)/*

# Relay deployment targets

SOLO_RELAY_HOST ?= tencent_gz_6
SOLO_RELAY_PORT ?= 8081
SOLO_RELAY_NGINX_PORT ?= 8081

deploy-solo-relay: solo-relay-linux-amd64
	@echo "Deploying solo relay to $(SOLO_RELAY_HOST)..."
	scp $(OUTPUT)/linux/solo-relay $(SOLO_RELAY_HOST):/opt/solo-relay/solo-relay
	ssh $(SOLO_RELAY_HOST) "chmod +x /opt/solo-relay/solo-relay && sudo systemctl restart solo-relay"
	@echo "Solo relay deployed and restarted on port $(SOLO_RELAY_PORT)"

# Relay selection targets for solo daemon

use-solo-relay:
	@echo "Configuring daemon to use solo relay (106.52.40.152:$(SOLO_RELAY_NGINX_PORT))..."
	@mkdir -p ~/.solo
	@if [ -f ~/.solo/config.json ]; then \
		cat ~/.solo/config.json | python3 -c "\
import json, sys; \
config = json.load(sys.stdin); \
config.setdefault('daemon', {}).setdefault('relay', {}); \
config['daemon']['relay']['enabled'] = True; \
config['daemon']['relay']['endpoint'] = '106.52.40.152:$(SOLO_RELAY_NGINX_PORT)'; \
config['daemon']['relay']['publicEndpoint'] = '106.52.40.152:$(SOLO_RELAY_NGINX_PORT)'; \
json.dump(config, sys.stdout, indent=2)" > ~/.solo/config.json.tmp && mv ~/.solo/config.json.tmp ~/.solo/config.json; \
	else \
		echo '{"daemon":{"relay":{"enabled":true,"endpoint":"106.52.40.152:$(SOLO_RELAY_NGINX_PORT)","publicEndpoint":"106.52.40.152:$(SOLO_RELAY_NGINX_PORT)"}}}' > ~/.solo/config.json; \
	fi
	@echo "Done. Restart daemon to apply: make restart"

relay-status:
	@echo "=== Solo Relay Status ==="
	ssh $(SOLO_RELAY_HOST) "sudo systemctl status solo-relay --no-pager | head -10"
	@echo ""
	@echo "=== Solo Relay Status ==="
	ssh $(SOLO_RELAY_HOST) "sudo systemctl status solo-relay --no-pager | head -10"
	@echo ""
	@echo "=== Current Daemon Config ==="
	@cat ~/.solo/config.json 2>/dev/null || echo "No config found"
