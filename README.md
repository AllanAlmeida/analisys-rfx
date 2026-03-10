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

## Swagger (OpenAPI)

- Arquivo: `docs/swagger.yaml`
- Pode ser importado no Swagger Editor ou usado para gerar client/server stubs.

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
  "index": "CDI",
  "modality": "POS",
  "maturity_date": "2026-08-13"
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

Campos adicionais no body:

- `modality` (opcional): `POS`, `PRE`, `IPCA` (default é inferido pelo tipo)
- `maturity_date` (opcional): formato `YYYY-MM-DD`
- `issuer` (opcional): emissor do produto (ex: `Banco BTG Pactual`)
- Para `LCI/LCA`, também é aceito `index=PREFIXADO` com `modality=PRE`

### `POST /analyze/batch`

Analisa uma lista de investimentos em uma única chamada.

Request:

```json
{
  "items": [
    {
      "type": "LCI",
      "rate": 11.77,
      "index": "PREFIXADO",
      "modality": "PRE",
      "maturity_date": "2026-09-10",
      "issuer": "Banco BTG Pactual"
    },
    {
      "type": "CDB",
      "rate": 120,
      "index": "CDI",
      "modality": "POS"
    }
  ]
}
```

### `POST /analyze/batch/from/plaintxt`

Recebe o texto bruto (copiado da corretora), transforma internamente em `items` e executa a análise em lote.

Aceita:

- `Content-Type: text/plain` com o texto direto no body
- `Content-Type: application/json` com `{ "text": "..." }`

No parse de plain text, o serviço extrai automaticamente o `issuer` de linhas como `LCI - Banco BTG Pactual`.

Response:

```json
{
  "parsed": 2,
  "parse_failed": 0,
  "parse_errors": [],
  "batch": {
    "total": 2,
    "ok": 2,
    "failed": 0,
    "items": [
      {
        "index": 0,
        "input": {
          "type": "LCI",
          "rate": 11.77,
          "index": "PREFIXADO",
          "modality": "PRE",
          "maturity_date": "2026-09-10",
          "issuer": "Banco BTG Pactual"
        },
        "result": {
          "classification": "Excepcional",
          "score": 10,
          "equivalent_cdb": 138.47,
          "equivalent_cdi_return": 117.7,
          "real_return": 6.96,
          "description": "LCI com 100% ou mais do CDI",
          "indicators": {
            "selic": 10.5,
            "ipca": 5.2,
            "cdi": 10
          }
        }
      }
    ]
  }
}
```

### `POST /analyze/batch/from/plaintxt/csv`

Mesmo comportamento do endpoint anterior, mas o retorno vem em `text/csv` para importacao em planilhas.

Colunas do CSV:

- `index`
- `type`
- `issuer`
- `rate`
- `indexer`
- `modality`
- `maturity_date`
- `classification`
- `score`
- `equivalent_cdb`
- `equivalent_cdi_return`
- `real_return`
- `description`
- `error`

Response:

```json
{
  "total": 2,
  "ok": 2,
  "failed": 0,
  "items": [
    {
      "index": 0,
      "input": {
        "type": "LCI",
        "rate": 11.77,
        "index": "PREFIXADO",
        "modality": "PRE",
        "maturity_date": "2026-09-10"
      },
      "result": {
        "classification": "Excepcional",
        "score": 9.9,
        "equivalent_cdb": 138.47,
        "equivalent_cdi_return": 117.7,
        "real_return": 6.96,
        "description": "LCI com 100% ou mais do CDI",
        "indicators": {
          "selic": 10.5,
          "ipca": 5.2,
          "cdi": 10.0
        }
      }
    }
  ]
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
  "index": "CDI",
  "modality": "POS",
  "maturity_date": "2026-08-13",
  "issuer": "Banco Fator S/A"
}
```

### LCI (Excepcional)

```json
{
  "type": "LCI",
  "rate": 102,
  "index": "CDI",
  "modality": "POS",
  "maturity_date": "2027-01-15",
  "issuer": "Banco BTG Pactual"
}
```

### LCA (Aceitável)

```json
{
  "type": "LCA",
  "rate": 87,
  "index": "CDI",
  "modality": "POS",
  "maturity_date": "2026-12-01"
}
```

### Tesouro Prefixado (Bom)

```json
{
  "type": "Tesouro Prefixado",
  "rate": 15.0,
  "index": "Prefixado",
  "modality": "PRE",
  "maturity_date": "2029-01-01"
}
```

### Tesouro IPCA+ (Aceitável)

```json
{
  "type": "Tesouro IPCA+",
  "rate": 5.4,
  "index": "IPCA",
  "modality": "IPCA",
  "maturity_date": "2035-05-15"
}
```

## Mais exemplos de curl

```bash
# Batch
curl -X POST http://localhost:8080/analyze/batch \
  -H "Content-Type: application/json" \
  -d '{
    "items":[
      {"type":"LCI","rate":11.77,"index":"PREFIXADO","modality":"PRE","maturity_date":"2026-09-10"},
      {"type":"LCA","rate":11.31,"index":"PREFIXADO","modality":"PRE","maturity_date":"2028-09-11"},
      {"type":"CDB","rate":120,"index":"CDI","modality":"POS"}
    ]
  }'

# Batch from plain text (text/plain)
curl -X POST http://localhost:8080/analyze/batch/from/plaintxt \
  -H "Content-Type: text/plain" \
  --data-binary $'LCI - Banco BTG Pactual\nPré-Fixado\nConservador\nJuros no vencimento\n10/09/2026\nPrazo: 184 dias\n11,77% a.a.\t14,81% a.a.\tR$ 1.000,00\t-\nInvestir'

# Batch from plain text (JSON)
curl -X POST http://localhost:8080/analyze/batch/from/plaintxt \
  -H "Content-Type: application/json" \
  -d '{"text":"LCI - Banco BTG Pactual\nPré-Fixado\nConservador\nJuros no vencimento\n10/09/2026\nPrazo: 184 dias\n11,77% a.a.\t14,81% a.a.\tR$ 1.000,00\t-\nInvestir"}'

# Batch from plain text -> CSV
curl -X POST http://localhost:8080/analyze/batch/from/plaintxt/csv \
  -H "Content-Type: text/plain" \
  --data-binary @lista.txt \
  -o analysis_batch.csv

# LCI
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"type":"LCI","rate":95,"index":"CDI","modality":"POS","maturity_date":"2026-08-13"}'

# Tesouro Prefixado
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"type":"Tesouro Prefixado","rate":14.7,"index":"Prefixado","modality":"PRE","maturity_date":"2029-01-01"}'

# Tesouro IPCA+
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  -d '{"type":"Tesouro IPCA+","rate":6.2,"index":"IPCA","modality":"IPCA","maturity_date":"2035-05-15"}'
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
