APPLICATION_PATH=main.go
REPORT_FILE=report.out
COVERAGE_FILE=coverage.out
GO_TEST_ENV=GOCACHE=/tmp/go-cache GOTMPDIR=/tmp/go-tmp
# Exclude generated mocks from coverage to avoid stale file references.
COVERPKG=$(shell ${GO_TEST_ENV} go list ./... | grep -v "/mocks" | paste -sd, -)
# Override SSH_KEY to use a specific Bitbucket key for local builds.
SSH_KEY?=$(HOME)/.ssh/id_rsa

.PHONY: run
run:
	@ go run ./cmd/api

.PHONY: mock
mock:
	@ rm -rf mocks
	@ go run github.com/vektra/mockery/v2@v2.53.5 --config=.mockery.yaml

.PHONY: build
build:
	@ GOOS=linux go build -o bin/application .

.PHONY: test
test:
	@ mkdir -p /tmp/go-cache /tmp/go-tmp
	@ ${GO_TEST_ENV} go test -count=1 -covermode=atomic -coverpkg ${COVERPKG} -coverprofile=${COVERAGE_FILE} ./... -json > ${REPORT_FILE}
	@ ${GO_TEST_ENV} go tool cover -func=${COVERAGE_FILE} | tail -n 1

.PHONY: coverage
coverage: test
	@ go tool cover -html=${COVERAGE_FILE}

