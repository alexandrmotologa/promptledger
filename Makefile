.PHONY: all build test clean ui run

all: build

ui:
	cd web && npm run build

build: ui
	go build -ldflags="-s -w" -o bin/promptledger.exe ./cmd/promptledger

test:
	go test -v -bench="BenchmarkRouter_Route" ./test

clean:
	rm -rf bin/ .promptledger/ web/dist/

run: build
	./bin/promptledger.exe serve --port 8080
