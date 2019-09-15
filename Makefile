build:
	mkdir -p bin
	cd server; GOOS=darwin GOARCH=amd64 go build -o ../bin/nodestatus-darwin-amd64
	cd server; GOOS=linux GOARCH=amd64 go build -o ../bin/nodestatus-linux-amd64

install:
	go install
