all: build

build:
	GOPATH=$$PWD go get "github.com/golang/glog"
	GOPATH=$$PWD go build -o $$PWD/bin/perf perf
