.PHONY: build test install

build:
	mkdir -p bin
	go build -o bin/taskboard ./cmd/taskboard
	ln -sf taskboard bin/tb

test:
	go test ./...

install:
	./scripts/install.sh
