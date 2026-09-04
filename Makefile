.PHONY: build build-web build-go run-server test test-go test-web clean

# web/dist is a build artifact (not committed) that pkg/server embeds via
# go:embed, so it must exist before the Go binaries can compile.
build: build-web build-go

build-web:
	cd web && npm install && npm run build

build-go:
	go build -o bin/oshimai-server ./cmd/server
	go build -o bin/oshimai        ./cmd/cli
	go build -o bin/oshimai-agent  ./cmd/agent

run-server: build
	./bin/oshimai-server -addr :8080

test: test-go test-web

test-go:
	go vet ./...
	go test ./...

test-web:
	cd web && npm run typecheck

clean:
	rm -rf bin web/dist
