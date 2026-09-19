.PHONY: run build test test-race vet fmt lint tidy docker clean

build:
	go build -o bin/goredis ./cmd/goredis

run: build
	./bin/goredis --port 6379 --aof db.aof

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

tidy:
	go mod tidy

docker:
	docker build -t goredis .

clean:
	rm -rf bin db.aof
