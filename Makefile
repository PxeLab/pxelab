.PHONY: build test run clean frontend

build:
	go build -o bin/pxelab ./cmd/pxelab

test:
	go test ./...

run:
	go run ./cmd/pxelab

clean:
	rm -rf bin/ dist/

frontend:
	cd web && npm ci && npm run build

release:
	goreleaser release --clean

release-snapshot:
	goreleaser release --clean --snapshot
