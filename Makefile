.PHONY: run test migrate-up migrate-down lint tidy

run:
	go run ./cmd/bot/...

test:
	go test ./...

migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

lint:
	@if [ "$$(gofmt -l . | wc -l)" -gt 0 ]; then \
		echo "Unformatted files:"; gofmt -l .; exit 1; \
	fi
	go vet ./...

tidy:
	go mod tidy
