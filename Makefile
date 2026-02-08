.PHONY: all build build-linux build-darwin build-ubuntu clean

BINARY_NAME=dnsspoofer
LDFLAGS=-ldflags="-s -w"

all: build

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .

build-ubuntu: build-linux
	@echo "Note: build-ubuntu is an alias for build-linux"
	@echo "For explicit Ubuntu build script, use: ./scripts/build-ubuntu.sh"

build-darwin:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .

build-all: build-linux build-darwin

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-linux-amd64 $(BINARY_NAME)-darwin-arm64

# Deploy to server (edit REMOTE as needed)
REMOTE=root@95.164.123.192
deploy: build-linux
	scp $(BINARY_NAME)-linux-amd64 $(REMOTE):/usr/local/bin/$(BINARY_NAME)
	ssh $(REMOTE) "chmod +x /usr/local/bin/$(BINARY_NAME)"
