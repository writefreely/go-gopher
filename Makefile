.PHONY: build install profile bench test clean

all: build

build:
	@go build -o client ./examples/client
	@go build -o fileserver ./examples/fileserver
	@go build -o hello ./examples/hello

install:
	@go install ./...

profile:
	@go test -cpuprofile cpu.prof -memprofile mem.prof -v -bench . .

bench:
	@go test -bench . .

test:
	@go test \
		-race \
		-cover \
		-coverprofile=coverage.txt \
		-covermode=atomic \
		.

clean:
	@git clean -f -d -X
