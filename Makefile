.PHONY: test
test:
	go test -v ./... -cover

.PHONY: test-coverage
test-coverage:
	go test -v ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

.PHONY: lint
lint:
	golangci-lint run

.PHONY: build
build:
	go build -o bin/app

.PHONY: run
run:
	go run main.go

.PHONY: docker-build
docker-build:
	docker-compose build

.PHONY: docker-up
docker-up:
	docker-compose up -d

.PHONY: docker-down
docker-down:
	docker-compose down