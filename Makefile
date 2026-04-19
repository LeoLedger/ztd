.PHONY: build run test lint docker-build docker-run clean load-test

BINARY_NAME=address-parse
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

docker-build:
	docker build -t $(BINARY_NAME):latest .

docker-run:
	docker run -p 8080:8080 --env-file .env $(BINARY_NAME):latest

load-test:
	./scripts/load_test.sh

clean:
	rm -rf $(BUILD_DIR)
