APPNAME := s3-helper

setup:
	go mod vendor

build:
	GOSUMDB=off GOPROXY=direct go build -o $(APPNAME)

test:
	go test ./... -v

setup-linters:
	go get -u github.com/alecthomas/gometalinter
	go get -u github.com/client9/misspell/cmd/misspell
	go get -u github.com/mdempsky/unconvert
	go get -u github.com/tsenart/deadcode
	go get -u golang.org/x/lint/golint
	go get -u honnef.co/go/tools/cmd/gosimple

run-linters:
	gometalinter ./... -e vendor
	
run:
	./$(APPNAME)
