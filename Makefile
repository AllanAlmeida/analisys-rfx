BINARY=bin/application
# Versao do go.mod, usada para fixar o toolchain das ferramentas externas.
GO_VERSION=$(shell sed -n 's/^go //p' go.mod)
COVERAGE_FILE=coverage.out
REPORT_FILE=report.out
GO_TEST_ENV=GOCACHE=/tmp/go-cache GOTMPDIR=/tmp/go-tmp
COVERPKG=$(shell ${GO_TEST_ENV} go list ./... | paste -sd, -)

.PHONY: run
run:
	@ go run ./cmd/api

.PHONY: build
build:
	@ go build -o ${BINARY} ./cmd/api
	@ echo "binario em ${BINARY}"

.PHONY: test
test:
	@ mkdir -p /tmp/go-cache /tmp/go-tmp
	@ ${GO_TEST_ENV} go test -count=1 -covermode=atomic -coverpkg ${COVERPKG} -coverprofile=${COVERAGE_FILE} ./... -json > ${REPORT_FILE}
	@ ${GO_TEST_ENV} go tool cover -func=${COVERAGE_FILE} | tail -n 1

.PHONY: coverage
coverage: test
	@ go tool cover -html=${COVERAGE_FILE}

.PHONY: lint
lint:
	@ test -z "$$(gofmt -l .)" || (echo "gofmt pendente em:" && gofmt -l . && exit 1)
	@ go vet ./...

.PHONY: race
race:
	@ ${GO_TEST_ENV} go test -race -count=1 ./...

# Complexidade ciclomatica do codigo de producao: nenhuma funcao acima de 10.
# Os _test.go ficam de fora: testes table-driven acumulam ramos nas assercoes,
# e o alvo aqui e a manutenibilidade do codigo que vai para producao.
.PHONY: cyclo
cyclo:
	@ go run github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0 -over 10 -ignore "_test\.go" . \
		&& echo "nenhuma funcao de producao acima de 10"

# Vulnerabilidades alcancaveis a partir deste codigo.
#
# GOTOOLCHAIN fixo e necessario: o x/vuln exige Go >= 1.26, entao `go run`
# escolhe o 1.26 para compilar a ferramenta, e um govulncheck compilado com
# 1.26 recusa analisar um modulo que pede 1.27 ("package requires newer Go
# version"). Fixando no que o go.mod declara, a ferramenta e o modulo usam a
# mesma versao.
.PHONY: vuln
vuln:
	@ GOTOOLCHAIN=go${GO_VERSION} go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

.PHONY: check
check: lint test cyclo vuln

.PHONY: docker
docker:
	@ docker build -t investment-analyzer .

.PHONY: clean
clean:
	@ rm -rf bin ${COVERAGE_FILE} ${REPORT_FILE}
