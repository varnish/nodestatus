deps:
	go get github.com/shirou/gopsutil
	go get -u github.com/jteeuwen/go-bindata/...
	go get github.com/elazarl/go-bindata-assetfs/...

build:
	go-bindata-assetfs static/...
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -o bin/nodestatus-darwin-amd64
	GOOS=linux GOARCH=amd64 go build -o bin/nodestatus-linux-amd64

install:
	go-bindata-assetfs static/...
	go install
