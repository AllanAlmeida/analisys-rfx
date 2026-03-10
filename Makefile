.PHONY: run test

run:
	go run ./cmd/api

test:
	go test ./...
