# Estágio 1: Compilação
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Copia os arquivos de dependências agora que eles existem localmente
COPY go.mod go.sum ./
RUN go mod download

# Copia o código e compila
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api

# Estágio 2: Imagem final leve
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]