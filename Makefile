build:
	go build -o ./bin/ridlfmt ./main.go

install:
	go install .

rerun-install:
	rerun -watch . -ignore out -run sh -c 'go install .'

rerun:
	rerun -watch . -ignore out -run sh -c 'go run . -s _examples/e1.ridl'

test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...
