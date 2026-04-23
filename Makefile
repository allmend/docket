.PHONY: dev build migrate sqlc js vet assets rebuild

assets:
	npx tailwindcss -i static/src/app.css -o static/dist/app.css --minify
	npx esbuild static/src/board.js --bundle --outdir=static/dist --minify

rebuild: assets vet build

dev:
	go run ./cmd/serve --mode=all

build:
	go build -o docket ./cmd/serve

migrate:
	go run ./cmd/serve --migrate-only

seed:
	go run ./cmd/seed

sqlc:
	sqlc generate

js:
	npx esbuild static/src/board.js --bundle --outdir=static/dist --minify

js-watch:
	npx esbuild static/src/board.js --bundle --outdir=static/dist --watch

css:
	npx tailwindcss -i static/src/app.css -o static/dist/app.css --minify

css-watch:
	npx tailwindcss -i static/src/app.css -o static/dist/app.css --watch

vet:
	go vet ./...

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down

dev-docker:
	docker compose -f docker/docker-compose.dev.yml up --build

dev-docker-down:
	docker compose -f docker/docker-compose.dev.yml down
