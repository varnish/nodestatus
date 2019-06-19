deps:
	go get github.com/shirou/gopsutil

build:
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -o bin/nodestatus-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o bin/nodestatus-linux-amd64

install:
	go install
