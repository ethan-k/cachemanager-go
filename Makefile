BINARY_NAME=cachemanager
GO_FILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")
TEST_DIRS=$(shell go list ./... | grep -v /vendor/)

GOLANGCI_LINT_VERSION=latest

.PHONY: test lint fmt install-lint clean

test:
	go test -v ./...

coverage:
	mkdir -p coverage
	go test -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

lint: install-lint
	golangci-lint run

install-lint:
	@which golangci-lint > /dev/null 2>&1 || { \
		echo "golangci-lint not found. Installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
		echo "golangci-lint installed."; \
	}

fmt:
	gofmt -l -w $(GO_FILES)
