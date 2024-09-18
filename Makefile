# Makefile for building the webhook project

BINARY_NAME := webhook
OUTPUT_DIR := bin

.PHONY: all build deps test clean

all: build

build: deps
	mkdir -p $(OUTPUT_DIR)
	go build -o $(OUTPUT_DIR)/$(BINARY_NAME) cmd/main.go

deps:
	go mod tidy

test:
	go test -v ./...

clean:
	rm -rf $(OUTPUT_DIR)
