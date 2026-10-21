VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo v0.0.0-dev)

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o ./bin/ridlfmt .

install:
	go install -ldflags="-s -w -X main.version=$(VERSION)" .

rerun-install:
	go run github.com/goware/rerun/cmd/rerun -watch . -ignore out -run sh -c 'make install'

rerun-1:
	go run github.com/goware/rerun/cmd/rerun -watch . -ignore out -run sh -c 'go run . -s _examples/e1.ridl'

rerun-2:
	go run github.com/goware/rerun/cmd/rerun -watch . -ignore out -run sh -c 'go run . -s _examples/e2.ridl'

test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...

coverage:
	go test -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt

coverage-html:
	go test -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html
