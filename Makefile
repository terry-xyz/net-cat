BINARY := TCPChat
ARGS ?=
COVERAGE_FILE := coverage.out

.PHONY: build run test test-v cover fmt clean

build:
	go build -o $(BINARY)

run: build
	./$(BINARY) $(ARGS)

test:
	go test ./...

test-v:
	go test -v ./...

cover:
	go test -coverprofile=$(COVERAGE_FILE) ./...
	go tool cover -func=$(COVERAGE_FILE)

fmt:
	gofmt -w .

clean:
	rm -f $(BINARY) $(BINARY).exe $(COVERAGE_FILE)
