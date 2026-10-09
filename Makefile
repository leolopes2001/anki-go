ifneq (,$(wildcard .env))
include .env
export
endif

API_PORT ?= 8080
WEB_PORT ?= 3000
DATA_DIR ?= ./data
STORAGE_DIR ?= ./storage

.PHONY: dev api web build test lint clean

dev:
	$(MAKE) -j2 api web

api:
	cd apps/api && DATA_DIR="$(abspath $(DATA_DIR))" STORAGE_DIR="$(abspath $(STORAGE_DIR))" go run ./cmd/server

web:
	cd apps/web && npm run dev -- --port $(WEB_PORT)

build:
	mkdir -p bin
	cd apps/api && go build -o ../../bin/anki-api ./cmd/server
	cd apps/web && npm run build

test:
	cd apps/api && go test ./...
	cd apps/web && npm run typecheck

lint:
	cd apps/api && gofmt -w . && go vet ./...
	cd apps/web && npm run lint

clean:
	rm -rf bin apps/web/.next