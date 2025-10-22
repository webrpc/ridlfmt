build:
	go build -o ./bin/ridlfmt ./main.go

install:
	go install .

rerun-install:
	go run github.com/goware/rerun/cmd/rerun -watch . -ignore out -run sh -c 'go install .'

rerun-1:
	go run github.com/goware/rerun/cmd/rerun -watch . -ignore out -run sh -c 'go run . -s _examples/e1.ridl'

rerun-2:
	go run github.com/goware/rerun/cmd/rerun -watch . -ignore out -run sh -c 'go run . -s _examples/e2.ridl'

test:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...
