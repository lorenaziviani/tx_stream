# Dockerfile para TxStream
FROM golang:1.21-alpine AS builder

# Instala dependências do sistema
RUN apk add --no-cache git ca-certificates tzdata

# Define o diretório de trabalho
WORKDIR /app

# Copia os arquivos de dependências
COPY go.mod go.sum ./

# Baixa as dependências
RUN go mod download

# Copia o código fonte
COPY . .

# Compila a aplicação
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o txstream ./cmd/txstream

# Imagem final
FROM alpine:latest

# Instala ca-certificates para HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Cria usuário não-root
RUN addgroup -g 1001 -S txstream && \
    adduser -u 1001 -S txstream -G txstream

# Define o diretório de trabalho
WORKDIR /app

# Copia o binário compilado
COPY --from=builder /app/txstream .

# Copia arquivos de migração
COPY --from=builder /app/migrations ./migrations

# Muda a propriedade dos arquivos
RUN chown -R txstream:txstream /app

# Muda para o usuário não-root
USER txstream

# Expõe a porta
EXPOSE 8080

# Define variáveis de ambiente padrão
ENV SERVER_PORT=8080
ENV SERVER_HOST=0.0.0.0

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Comando para executar a aplicação
CMD ["./txstream"] 