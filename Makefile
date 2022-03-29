APPNAME := s3-helper

# 'all' is the default target
all: clean build setup-specs run-specs

clean:
	rm -f $(APPNAME)
	find . -type f -name "*.coverprofile" -delete
	rm -rf vendor/
	go mod tidy

setup:
	go mod vendor

build:
	GOSUMDB=off GOPROXY=direct GOOS=linux GOARCH=amd64 go build -o $(APPNAME)

test:
	go test ./... -v

run-linters:
	gofmt -w -s .

# should be used only once in the local machine
setup-specs:
	go install github.com/onsi/ginkgo/v2/ginkgo@latest
	go install github.com/onsi/gomega/...

# for running the unit tests in the local machine as well as in the build machine
run-specs:
	ginkgo -r --randomize-all --randomize-suites --cover --race --trace
	go vet

test-coverage:
	go build -o $(APPNAME)
	ginkgo -r --randomize-all --randomize-suites --cover -coverprofile=coverage.out --race --trace
	go tool cover -html=coverage.out

run:
	./$(APPNAME)

.PHONY: clean all compose
