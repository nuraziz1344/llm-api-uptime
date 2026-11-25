.PHONY: dev-backend dev-frontend build test docker-build

dev-backend:
	DEV_MODE=true DATA_DIR=./data go run ./cmd/server

dev-frontend:
	cd web && pnpm dev

build:
	cd web && pnpm build
	rm -rf internal/webui/dist
	cp -r web/dist internal/webui/dist
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/llm-api-uptime ./cmd/server

test:
	go test -race ./...
	cd web && pnpm exec tsc --noEmit

docker-build:
	docker build -t llm-api-uptime:local .
