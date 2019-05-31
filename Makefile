APPNAME := s3-helper

# 'all' is the default target
all: setup build setup-specs run-specs
ci-all: build setup-specs run-specs

clean:
	rm $(APPNAME)

setup:
	go get -d github.com/golang/dep

build:
	dep ensure
	go build -o $(APPNAME)

setup-linters:
	go get -u github.com/alecthomas/gometalinter
	go get -u github.com/client9/misspell/cmd/misspell
	go get -u github.com/mdempsky/unconvert
	go get -u github.com/tsenart/deadcode
	go get -u golang.org/x/lint/golint
	go get -u honnef.co/go/tools/cmd/gosimple

run-linters:
	gometalinter ./... -e vendor

# should be used only once in the local machine
setup-specs:
	go get -u -t github.com/onsi/ginkgo/ginkgo
	go get -u -t github.com/onsi/gomega/...

# for running the unit tests in the local machine as well as in the build machine
run-specs:
	ginkgo -r --randomizeAllSpecs --randomizeSuites --failOnPending --cover --trace --race --progress
	go vet

run:
	./$(APPNAME)

.PHONY: clean all
