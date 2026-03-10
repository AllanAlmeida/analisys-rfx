# Investment Analyzer API

Microserviço REST em Go para analisar investimentos de renda fixa brasileiros e classificá-los em:

- Excepcional
- Bom
- Aceitável
- Fraco

Tipos suportados:

- CDB
- LCI
- LCA
- Tesouro Prefixado
- Tesouro IPCA+

O serviço também calcula:

- Equivalência em CDB
- Retorno equivalente em CDI
- Retorno real versus inflação
- Score de investimento de 0 a 10

Os indicadores econômicos (SELIC, IPCA e CDI) são buscados na API do Banco Central e armazenados em cache por 1 hora.

## Stack

- Go
- `net/http`
- `encoding/json`
- `context`
- `log`
- `time`
- `github.com/go-chi/chi/v5`

## Estrutura do projeto

```text
investment-analyzer/
├── cmd/api/main.go
├── internal/domain/investment.go
├── internal/service/analyzer.go
├── internal/service/economy_service.go
├── internal/handler/investment_handler.go
├── internal/router/router.go
├── pkg/utils/calculator.go
├── Dockerfile
└── README.md
```

## Endpoints

### `GET /health`

Health check.

### `POST /analyze`

Analisa um investimento e retorna classificação, score e métricas.

Request:

```json
{
  "type": "LCI",
  "rate": 95,
  "index": "CDI"
}
```

Response:

```json
{
  "classification": "Bom",
  "score": 8.3,
  "equivalent_cdb": 111.76,
  "equivalent_cdi_return": 95,
  "real_return": 3.85,
  "description": "LCI entre 90% e 99% do CDI",
  "indicators": {
    "selic": 10.5,
    "ipca": 5.2,
    "cdi": 10.4
  }
}
```

## Regras de classificação

### CDB

- `>= 120% CDI`: Excepcional
- `105% - 119% CDI`: Bom
- `100% - 104% CDI`: Aceitável
- `< 100% CDI`: Fraco

### LCI / LCA

- `>= 100% CDI`: Excepcional
- `90% - 99% CDI`: Bom
- `85% - 89% CDI`: Aceitável
- `< 85% CDI`: Fraco

### Tesouro Prefixado

- `>= 15.5%`: Excepcional
- `14.5% - 15.4%`: Bom
- `13.5% - 14.4%`: Aceitável
- `< 13.5%`: Fraco

### Tesouro IPCA+

- `>= 6.5%`: Excepcional
- `5.8% - 6.4%`: Bom
- `5.0% - 5.7%`: Aceitável
- `< 5.0%`: Fraco

## Execução local

Pré-requisito: Go 1.22+

```bash
go run ./cmd/api
```

Variáveis de ambiente:

- `PORT` (default: `8080`)

## Exemplo com curl

```bash
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "type":"CDB",
    "rate":120,
    "index":"CDI"
  }'
```

## Variações de body (POST /analyze)

### CDB (Bom)

```json
{
  "type": "CDB",
  "rate": 108,
  "index": "CDI"
}
```

### LCI (Excepcional)

```json
{
  "type": "LCI",
  "rate": 102,
  "index": "CDI"
}
```

### LCA (Aceitável)

```json
{
  "type": "LCA",
  "rate": 87,
  "index": "CDI"
}
```

### Tesouro Prefixado (Bom)

```json
{
  "type": "Tesouro Prefixado",
  "rate": 15.0,
  "index": "Prefixado"
}
```

### Tesouro IPCA+ (Aceitável)

```json
{
  "type": "Tesouro IPCA+",
  "rate": 5.4,
  "index": "IPCA"
}
```

## Mais exemplos de curl

```bash
# LCI
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"type":"LCI","rate":95,"index":"CDI"}'

# Tesouro Prefixado
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"type":"Tesouro Prefixado","rate":14.7,"index":"Prefixado"}'

# Tesouro IPCA+
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"type":"Tesouro IPCA+","rate":6.2,"index":"IPCA"}'
```

## Docker

Build:

```bash
docker build -t investment-analyzer .
```

Run:

```bash
docker run --rm -p 8080:8080 -e PORT=8080 investment-analyzer
```

## Deploy no Render

### Opção 1: Web Service com Go

1. Suba o código em um repositório Git.
2. No Render, crie um novo `Web Service`.
3. Configure:
   - Runtime: `Go`
   - Build Command: `go build -o bin/api ./cmd/api`
   - Start Command: `./bin/api`
4. Adicione variável de ambiente:
   - `PORT=8080`
5. Faça deploy.

### Opção 2: Web Service com Docker

1. Crie um novo `Web Service` apontando para o repositório.
2. Selecione `Docker` como ambiente.
3. Render detectará o `Dockerfile` automaticamente.
4. Configure variável:
   - `PORT=8080`
5. Faça deploy.
