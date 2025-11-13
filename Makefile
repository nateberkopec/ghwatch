.PHONY: all build test clean install integration-test

# Default target
all: build

# Build the binary
build:
	go build -o ghwatch ./cmd/ghwatch

# Run unit/snapshot tests
test:
	go test ./...

# Run integration tests
integration-test:
	go test -tags=integration ./integration

# Clean build artifacts
clean:
	rm -f ghwatch

# Install to ~/.local/bin
install: build
	mkdir -p ~/.local/bin
	mv ghwatch ~/.local/bin/

# Install to /usr/local/bin (requires sudo)
install-system: build
	sudo install -m 755 ghwatch /usr/local/bin/
