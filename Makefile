.PHONY: build test run-serve run-play migrate-up migrate-down sqlc fmt vet lint tidy clean install

BINARY=tarmy
PKG=./...
DB_URL?=$(shell grep -E '^DATABASE_URL=' .env 2>/dev/null | cut -d= -f2-)

build:
	go build -o $(BINARY) ./cmd/tarmy

install:
	go install ./cmd/tarmy

run-serve: build
	./$(BINARY) serve

run-play: build
	./$(BINARY) play

migrate-up:
	migrate -path internal/store/migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path internal/store/migrations -database "$(DB_URL)" down 1

migrate-create:
	migrate create -ext sql -dir internal/store/migrations -seq $(name)

sqlc:
	sqlc generate

test:
	go test -race -count=1 $(PKG)

test-cover:
	go test -race -count=1 -coverprofile=coverage.out $(PKG)
	go tool cover -html=coverage.out -o coverage.html

fmt:
	gofmt -s -w .

vet:
	go vet $(PKG)

tidy:
	go mod tidy

lint: fmt vet

clean:
	rm -f $(BINARY) coverage.out coverage.html
