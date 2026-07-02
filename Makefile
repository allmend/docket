.PHONY: dev build migrate js js-watch css css-watch vet test test-short assets vendor rebuild docker-up docker-down dev-docker dev-docker-down

vendor:
	node scripts/vendor.js

assets: vendor
	npx tailwindcss -i static/src/app.css -o static/dist/app.css --minify
	npx esbuild static/src/board.js static/src/editor.js --bundle --outdir=static/dist --minify

rebuild: assets vet build

dev:
	go run ./cmd/serve --mode=all

build:
	go build -o docket ./cmd/serve

migrate:
	go run ./cmd/serve --migrate-only

js:
	npx esbuild static/src/board.js static/src/editor.js --bundle --outdir=static/dist --minify

js-watch:
	npx esbuild static/src/board.js static/src/editor.js --bundle --outdir=static/dist --watch

css:
	npx tailwindcss -i static/src/app.css -o static/dist/app.css --minify

css-watch:
	npx tailwindcss -i static/src/app.css -o static/dist/app.css --watch

vet:
	go vet ./...

test:
	go test ./...

# Fast inner loop — skips store integration tests (no Docker/Postgres needed).
test-short:
	go test -short ./...

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down

dev-docker:
	docker compose -f docker/docker-compose.dev.yml up --build

dev-docker-down:
	docker compose -f docker/docker-compose.dev.yml down
