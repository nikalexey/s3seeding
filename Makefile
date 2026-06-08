.PHONY: build gen bench all seed test fmt vet tidy clean

BIN := s3seeding
CONFIG ?= config.json
STORAGE ?= zakroma_01

build:
	go build -o $(BIN) ./cmd/s3seeding

seed:
	go run ./cmd/s3seeding seed -config $(CONFIG) -storage $(STORAGE)

test:
	go test -race ./...
