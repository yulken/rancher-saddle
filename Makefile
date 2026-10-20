BINARY := rancher-tui
COVERFILE := coverage.out

.PHONY: build test test-cover test-verbose lint clean

build:
	go build -o $(BINARY)

test:
	go test ./...

test-cover:
	go test -coverprofile=$(COVERFILE) ./...
	go tool cover -func=$(COVERFILE)

test-verbose:
	go test -v ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY) $(COVERFILE)
