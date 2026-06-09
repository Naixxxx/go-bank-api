.PHONY: run test fmt docker-up docker-down tidy

run:
	go run ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go')

tidy:
	go mod tidy

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down
