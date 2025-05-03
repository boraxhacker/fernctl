.PHONY: clean build

BINARY_NAME=fernctl

build: clean
	mkdir -p release
	GOOS=linux GOARCH=amd64 go build -o release/${BINARY_NAME}-amd64 ./cmd/${BINARY_NAME}

clean:
	go clean
	rm -rf release