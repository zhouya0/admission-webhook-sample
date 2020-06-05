BIN_DIR=_output/bin

all:local

init:
	mkdir -p ${BIN_DIR}

local: init
	go build -o=${BIN_DIR}/admission-webhook cmd/main.go

build-linux: init
	GO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o=${BIN_DIR}/admission-webhook cmd/main.go

image: build-linux
	docker build --no-cache . -t admission-webhook

update:
	go mod download
	go mod tidy
	go mod vendor

clean:
	rm -rf _output/
	rm -f *.log