# There are four main groups of operations provided by this Makefile: build,
# clean, run and tasks.

# Build operations will create artefacts from code. This includes things such as
# binaries, mock files, or catalogs of CNF tests.

# Clean operations remove the results of the build tasks, or other files not
# considered permanent.

# Run operations provide shortcuts to execute built binaries in common
# configurations or with default options. They are part convenience and part
# documentation.

# Tasks provide shortcuts to common operations that occur frequently during
# development. This includes running configured linters and executing unit tests

GO_PATH=$(shell go env GOPATH)
GO_PACKAGES=$(shell go list ./... | grep -v vendor)

.PHONY:	build \
	clean \
	lint \
	test \
	build-oct \
	vet

OCT_TOOL_NAME=oct
GOLANGCI_VERSION=v1.64.3

# Run the unit tests and build all binaries
build:
	make lint
	make test
	make build-oct

build-oct:
	go build -o oct -v cmd/tnf/main.go

build-oct-debug:
	go build -gcflags "all=-N -l" -extldflags '-z relro -z now' ./${OCT_TOOL_NAME}

# Cleans up auto-generated and report files
clean:
	go clean
	rm -f ./${OCT_TOOL_NAME}
	rm -f cover.out

test:
	UNIT_TEST="true" go test -coverprofile=cover.out ./...

# Run configured linters
lint:
	golangci-lint run --timeout 5m0s

coverage-html: test
	cat cover.out.tmp | grep -v "_moq.go" > cover.out
	go tool cover -html cover.out

update-certified-catalog:
	./oct fetch --operator --container --helm

# Update source dependencies and fix versions
update-deps:
	go mod tidy && \
	go mod vendor

# Install golangci-lint	
install-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GO_PATH}/bin ${GOLANGCI_VERSION}

vet:
	go vet ${GO_PACKAGES}
