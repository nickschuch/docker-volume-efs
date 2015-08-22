#!/usr/bin/make -f

GO=go
GB=gb

all: test

build: deps
	@echo "Building..."
	@$(GB) build all

deps:
	@echo "Installing gb..."
	@$(GO) get github.com/constabulary/gb/...

test: build
	@echo "Running tests..."
	@$(GB) test -test.v=true
