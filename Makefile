.PHONY: build test run clean frontend

build:
	go build -o bin/pxego ./cmd/pxego

test:
	go test ./...

run:
	go run ./cmd/pxego

clean:
	rm -rf bin/

frontend:
	cd web && npm ci && npm run build
