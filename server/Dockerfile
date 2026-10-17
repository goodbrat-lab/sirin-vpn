# Stage 1: Build all 3 binaries
FROM golang:1.27-alpine AS builder

# Зависимости для CGO (нужны для tun)
RUN apk add --no-cache gcc musl-dev linux-headers git

WORKDIR /build

# Копируем go.mod сначала (кэш слоёв)
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходник
COPY . .

# Собираем 3 бинарника (static, без CGO зависимостей где возможно)
# sirind требует CGO для tun? Нет — songgao/water использует syscall напрямую
# Но всё равно собираем с CGO_ENABLED=0 для статики
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go build -ldflags="-s -w" -o /out/sirind ./cmd/sirind
RUN go build -ldflags="-s -w" -o /out/sirinapi ./cmd/sirinapi
RUN go build -ldflags="-s -w" -o /out/sirinbot ./cmd/sirinbot

# Stage 2: Runtime
FROM alpine:3.20

# ca-certificates — для HTTPS (sirinbot → api.telegram.org)
# tzdata — для корректного времени в логах
# iproute2 — для ip link/addr (sirind)
RUN apk add --no-cache ca-certificates tzdata iproute2

# Копируем бинарники
COPY --from=builder /out/sirind /usr/local/bin/sirind
COPY --from=builder /out/sirinapi /usr/local/bin/sirinapi
COPY --from=builder /out/sirinbot /usr/local/bin/sirinbot

# Директории для логов и данных
RUN mkdir -p /var/log/sirin /etc/sirin

# Метка сервиса будет задана в compose (command)
WORKDIR /opt/sirin

# По умолчанию — показать справку (command переопределяется в compose)
CMD ["sirind", "--help"]
