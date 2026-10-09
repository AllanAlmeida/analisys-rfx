FROM golang:1.27.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# Sem GOARCH fixo: a imagem passa a construir para a arquitetura do host, em vez
# de produzir sempre um binario amd64 que nao roda em ARM.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/investment-analyzer ./cmd/api

FROM alpine:3.24

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /bin/investment-analyzer /app/investment-analyzer

ENV PORT=8080
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/health" || exit 1

USER app
CMD ["/app/investment-analyzer"]
