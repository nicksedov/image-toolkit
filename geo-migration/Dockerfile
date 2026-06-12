# ---- Stage 1: Build the Go binary (Linux) ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o geo-migration .

# ---- Stage 2: Runtime image ----
FROM alpine:3.21

COPY --from=builder /build/geo-migration /usr/local/bin/geo-migration

WORKDIR /app

# The app expects .env at this relative path
COPY .env .

ENTRYPOINT ["geo-migration"]
