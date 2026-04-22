.PHONY: build run clean test format

K6_BIN := k6
XK6_BIN := $(shell go env GOPATH)/bin/xk6

build:
	@echo "Installing xk6..."
	@go install go.k6.io/xk6/cmd/xk6@latest
	@echo "Building custom k6 binary with xk6-sip-media extension..."
	@$(XK6_BIN) build --with xk6-sip-media=.

run: build
	@echo "Running root script.js..."
	./$(K6_BIN) run script.js

test:
	@echo "Running tests..."
	CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-w" go test -v -race -count=1 ./...

format:
	@echo "Formatting code..."
	go fmt ./...

clean:
	@echo "Cleaning up..."
	rm -f $(K6_BIN)
	rm -f examples/audio/*.wav examples/audio/*.mp3
	rm -f *.pcap
